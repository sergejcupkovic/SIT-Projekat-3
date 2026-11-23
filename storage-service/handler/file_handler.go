package handler

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"storage-service/domain"
	"storage-service/filecheck"
	"storage-service/service"

	storagepb "stashr/proto/storage"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// What the upload needs from its collaborators. The real implementations are
// repository.FileRepository / FolderRepository, service.ObjectStore and
// service.QuotaClient; tests plug in fakes.
type (
	fileRepository interface {
		Create(ctx context.Context, file *domain.FileItem) error
		ExistsByNameInFolder(ctx context.Context, ownerID string, folderID *primitive.ObjectID, name string) (bool, error)
		FindByFolder(ctx context.Context, ownerID string, folderID *primitive.ObjectID) ([]domain.FileItem, error)
	}
	folderLookup interface {
		FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Folder, error)
	}
	objectStore interface {
		Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
		Delete(ctx context.Context, key string) error
	}
	quotaService interface {
		Get(ctx context.Context) (service.Quota, error)
		Reserve(ctx context.Context, sizeBytes int64) (service.Quota, error)
		Release(ctx context.Context, sizeBytes int64)
	}
)

// FileHandler implements the file related rpcs of storagepb.StorageServiceServer
// (UploadFile, GetQuota); StorageServer glues it together with FolderHandler.
type FileHandler struct {
	files        fileRepository
	folders      folderLookup
	objects      objectStore
	quota        quotaService
	maxFileBytes int64
}

// NewFileHandler: maxFileBytes <= 0 means "no per-file limit" (the quota still applies).
func NewFileHandler(files fileRepository, folders folderLookup, objects objectStore, quota quotaService, maxFileBytes int64) *FileHandler {
	return &FileHandler{files: files, folders: folders, objects: objects, quota: quota, maxFileBytes: maxFileBytes}
}

// headSize is how many leading bytes filecheck needs to sniff the content type.
const headSize = 512

// UploadFile  <-  POST /api/files/upload  (multipart; the api-gateway turns it into this stream)
//
// Order of events (US-2.2):
//
//	2.2.1  the first stream message carries the file name, size and target folder
//	       (the user picked both in the UI); the folder must exist and be the caller's.
//	       Cheap checks run first: size, name/extension whitelist, content sniffing,
//	       duplicate name. Nothing is stored and no quota is touched yet.
//	2.2.2  BEFORE saving, the Users service is asked synchronously whether the file
//	       fits into the caller's quota; the space is reserved in the same atomic step.
//	2.2.4  no room -> codes.ResourceExhausted with a clear message, nothing is stored.
//	2.2.3  otherwise the content goes to the object store, and name, type, size,
//	       owner, parent folder and timestamp go to MongoDB.
//
// If anything fails after the reservation, the stored object and the reserved
// quota are rolled back, so a failed upload never costs the user any space.
func (h *FileHandler) UploadFile(stream storagepb.StorageService_UploadFileServer) error {
	ctx := stream.Context()
	userID := userIDFrom(ctx)

	// ---- 2.2.1: what and where ----
	first, err := stream.Recv()
	if err != nil {
		return streamError(err, "Upload stream ended before the file details were received")
	}
	meta := first.GetMetadata()
	if meta == nil {
		return status.Error(codes.InvalidArgument, "The first upload message must contain the file details")
	}

	name := strings.TrimSpace(meta.GetFileName())
	size := meta.GetSizeBytes()
	if size <= 0 {
		return status.Error(codes.InvalidArgument, "Cannot upload an empty file (0 bytes)")
	}
	if h.maxFileBytes > 0 && size > h.maxFileBytes {
		return status.Errorf(codes.InvalidArgument, "File is too large: the maximum size of a single file is %s", humanBytes(h.maxFileBytes))
	}

	// Read the first bytes: the content must match the extension (a renamed
	// executable is rejected) and this is also how the type is determined,
	// instead of trusting the Content-Type the client claims.
	chunks := &chunkReader{stream: stream, limit: size}
	content := bufio.NewReaderSize(chunks, 64<<10)
	head, err := content.Peek(headSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return streamError(err, "Failed to read the file content")
	}
	if err := filecheck.ValidateUpload(name, head); err != nil {
		return status.Error(codes.InvalidArgument, uploadRejectedMessage(err))
	}
	mimeType := filecheck.DetectType(head)

	folderID, err := h.targetFolder(ctx, userID, meta)
	if err != nil {
		return err
	}

	exists, err := h.files.ExistsByNameInFolder(ctx, userID, folderID, name)
	if err != nil {
		return status.Error(codes.Internal, "Error checking file uniqueness")
	}
	if exists {
		return status.Error(codes.AlreadyExists, duplicateFileMsg)
	}

	// ---- 2.2.2: synchronous quota check with the Users service, before saving ----
	quota, err := h.quota.Reserve(ctx, size)
	if err != nil {
		return quotaServiceError(err)
	}
	// ---- 2.2.4: quota violation aborts the upload with a clear message ----
	if !quota.Allowed {
		return status.Error(codes.ResourceExhausted, quotaExceededMessage(size, quota))
	}

	// From here on the space is reserved: undo everything unless we commit.
	fileID := primitive.NewObjectID()
	objectKey := userID + "/" + fileID.Hex()
	committed := false
	defer func() {
		if committed {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := h.objects.Delete(cleanupCtx, objectKey); err != nil {
			log.Printf("WARNING: could not remove object %s after a failed upload: %v", objectKey, err)
		}
		h.quota.Release(cleanupCtx, size)
	}()

	// ---- 2.2.3: content -> object store ----
	hash := sha256.New()
	if err := h.objects.Put(ctx, objectKey, io.TeeReader(content, hash), size, mimeType); err != nil {
		if chunks.err != nil { // the client's stream failed or misbehaved, not the store
			return chunks.err
		}
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return err
		}
		if chunks.received < size {
			return status.Error(codes.InvalidArgument, "File content is shorter than the declared size")
		}
		log.Printf("object store put %s: %v", objectKey, err)
		return status.Error(codes.Internal, "Failed to store file content")
	}
	// Wait for the client to finish the stream and make sure it did not send
	// more (or less) than it declared.
	if _, err := io.Copy(io.Discard, content); err != nil {
		return streamError(err, "Failed to read the file content")
	}
	if chunks.received != size {
		return status.Error(codes.InvalidArgument, "File content does not match the declared size")
	}

	// ---- 2.2.3: metadata -> document database ----
	file := &domain.FileItem{
		ID:        fileID,
		Name:      name,
		FolderID:  folderID,
		OwnerID:   userID,
		SizeBytes: size,
		MimeType:  mimeType,
		ObjectKey: objectKey,
		SHA256:    hex.EncodeToString(hash.Sum(nil)),
		CreatedAt: time.Now(),
	}
	if err := h.files.Create(ctx, file); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return status.Error(codes.AlreadyExists, duplicateFileMsg)
		}
		log.Printf("save file metadata %s: %v", objectKey, err)
		return status.Error(codes.Internal, "Failed to save file metadata")
	}
	committed = true

	return stream.SendAndClose(&storagepb.UploadFileResponse{
		Message: "File uploaded successfully",
		File:    toProtoFile(file),
		Quota:   toProtoQuota(quota),
	})
}

// GetQuota  <-  GET /api/files/quota
func (h *FileHandler) GetQuota(ctx context.Context, _ *storagepb.GetQuotaRequest) (*storagepb.StorageQuota, error) {
	quota, err := h.quota.Get(ctx)
	if err != nil {
		return nil, quotaServiceError(err)
	}
	return toProtoQuota(quota), nil
}

// targetFolder resolves the folder the user picked; nil means the root. The
// folder must exist and belong to the caller.
func (h *FileHandler) targetFolder(ctx context.Context, userID string, meta *storagepb.UploadFileMetadata) (*primitive.ObjectID, error) {
	raw := strings.TrimSpace(meta.GetFolderId())
	if raw == "" {
		return nil, nil
	}

	oid, err := primitive.ObjectIDFromHex(raw)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid target folder ID")
	}
	folder, err := h.folders.FindByID(ctx, oid)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, "Target folder not found")
		}
		return nil, status.Error(codes.Internal, "Error verifying target folder")
	}
	if folder.OwnerID != userID {
		return nil, status.Error(codes.PermissionDenied, "Access denied to target folder")
	}
	return &oid, nil
}

// chunkReader presents the chunks of the upload stream as one io.Reader. It
// refuses to deliver more bytes than the size declared up front, so a client
// cannot reserve quota for a small file and then stream a huge one.
type chunkReader struct {
	stream   storagepb.StorageService_UploadFileServer
	limit    int64
	received int64
	buf      []byte
	done     bool
	err      error // first error seen; returned from then on
}

func (c *chunkReader) Read(p []byte) (int, error) {
	for len(c.buf) == 0 {
		if c.err != nil {
			return 0, c.err
		}
		if c.done {
			return 0, io.EOF
		}

		msg, err := c.stream.Recv()
		if errors.Is(err, io.EOF) {
			c.done = true
			return 0, io.EOF
		}
		if err != nil {
			c.err = streamError(err, "Upload stream failed")
			return 0, c.err
		}

		chunk, ok := msg.GetPayload().(*storagepb.UploadFileRequest_Chunk)
		if !ok {
			c.err = status.Error(codes.InvalidArgument, "Unexpected message in the upload stream: file details may only be sent once, first")
			return 0, c.err
		}
		if c.received+int64(len(chunk.Chunk)) > c.limit {
			c.err = status.Error(codes.InvalidArgument, "File content is larger than the declared size")
			return 0, c.err
		}
		c.received += int64(len(chunk.Chunk))
		c.buf = chunk.Chunk
	}

	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

// #region helperi

const duplicateFileMsg = "A file with this name already exists in the current folder"

// streamError keeps a gRPC status the stream already carries (Canceled,
// DeadlineExceeded, ...) and wraps anything else.
func streamError(err error, msg string) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Internal, msg)
}

// quotaServiceError: a failing Users service must never let an upload through
// unchecked, so every failure aborts it. Authentication problems keep their
// meaning; everything else is reported as "service unavailable".
func quotaServiceError(err error) error {
	switch status.Code(err) {
	case codes.Unauthenticated, codes.PermissionDenied, codes.NotFound:
		return err
	}
	log.Printf("quota check failed: %v", err)
	return status.Error(codes.Unavailable, "Could not verify your storage quota with the Users service. Nothing was uploaded, please try again.")
}

func quotaExceededMessage(size int64, q service.Quota) string {
	return fmt.Sprintf(
		"Storage quota exceeded: this file is %s but only %s is free (%s of %s used). The upload was aborted and nothing was saved.",
		humanBytes(size), humanBytes(q.RemainingBytes), humanBytes(q.UsedBytes), humanBytes(q.QuotaBytes))
}

func uploadRejectedMessage(err error) string {
	switch {
	case errors.Is(err, filecheck.ErrBadName):
		return "Invalid file name: use letters, digits, spaces and . _ - ( ) only (max 255 characters)"
	case errors.Is(err, filecheck.ErrTypeNotAllowed):
		return "This file type is not allowed. Allowed extensions: " + strings.Join(filecheck.AllowedExtensions(), ", ")
	case errors.Is(err, filecheck.ErrContentMismatch):
		return "The file content does not match its extension"
	}
	return "The file was rejected"
}

// humanBytes renders 1536 as "1.5 KB".
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

func toProtoFile(f *domain.FileItem) *storagepb.FileItem {
	return &storagepb.FileItem{
		Id:        f.ID.Hex(),
		Name:      f.Name,
		SizeBytes: f.SizeBytes,
		MimeType:  f.MimeType,
		CreatedAt: timestamppb.New(f.CreatedAt),
	}
}

func toProtoQuota(q service.Quota) *storagepb.StorageQuota {
	return &storagepb.StorageQuota{
		StorageQuotaBytes: q.QuotaBytes,
		UsedStorageBytes:  q.UsedBytes,
		RemainingBytes:    q.RemainingBytes,
	}
}

// #endregion
