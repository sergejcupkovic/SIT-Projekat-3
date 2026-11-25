package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	storagepb "stashr/proto/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// File upload is the one route that is NOT generated from a google.api.http
// annotation: grpc-gateway cannot decode multipart/form-data. This handler is
// registered on the same mux (gw.HandlePath), so it still sits behind CORS,
// security headers and the rate limiter, and answers in the same JSON shape.
//
//	POST /api/files/upload   multipart/form-data
//	     file      the file (required)
//	     folderId  target folder; omit for the root "My Drive"
//
// It turns the request into the client-streaming rpc StorageService.UploadFile:
// one message with name, size and folder, then the content in 64 KiB chunks.
// The size sent up front is the REAL size of the received file, which is what
// lets storage-service check the quota before it stores a single byte.

const (
	uploadChunkSize = 64 << 10
	// Room for the multipart framing and the folderId field on top of the file itself.
	multipartOverhead = 1 << 20
	// Parts bigger than this are spooled to temporary files instead of RAM.
	multipartMemory = 8 << 20
	// A big upload over a slow link may legitimately take a while; this replaces
	// the server-wide 30 s read/write timeouts for this one route.
	uploadTimeout = 10 * time.Minute
)

type uploadHandler struct {
	gw        *runtime.ServeMux
	storage   storagepb.StorageServiceClient
	jwtSecret []byte
	maxBytes  int64
}

func newUploadHandler(gw *runtime.ServeMux, storage storagepb.StorageServiceClient, jwtSecret []byte, maxFileBytes int64) *uploadHandler {
	return &uploadHandler{gw: gw, storage: storage, jwtSecret: jwtSecret, maxBytes: maxFileBytes}
}

// handle serves POST /api/files/upload.
func (u *uploadHandler) handle(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	authHeader := r.Header.Get("Authorization")
	if !u.validToken(authHeader) {
		writeError(w, http.StatusUnauthorized, codes.Unauthenticated, "Missing or invalid Authorization header")
		return
	}

	file, header, cleanup, ok := u.parseMultipartFile(w, r)
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()

	meta := &storagepb.UploadFileMetadata{
		FileName:  header.Filename,
		SizeBytes: header.Size,
	}
	if vals := r.MultipartForm.Value["folderId"]; len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
		folderID := strings.TrimSpace(vals[0])
		meta.FolderId = &folderID
	}

	ctx, cancel := context.WithTimeout(r.Context(), uploadTimeout)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeader)

	resp, err := u.stream(ctx, meta, file)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	u.writeProtoResponse(w, r, http.StatusCreated, resp)
}

// handleVersion serves POST /api/files/{file_id}/version.
func (u *uploadHandler) handleVersion(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
	authHeader := r.Header.Get("Authorization")
	if !u.validToken(authHeader) {
		writeError(w, http.StatusUnauthorized, codes.Unauthenticated, "Missing or invalid Authorization header")
		return
	}

	fileID := strings.TrimSpace(pathParams["file_id"])
	if fileID == "" {
		writeError(w, http.StatusBadRequest, codes.InvalidArgument, "Missing target file ID")
		return
	}

	file, header, cleanup, ok := u.parseMultipartFile(w, r)
	if !ok {
		return
	}
	defer cleanup()
	defer file.Close()

	meta := &storagepb.UploadNewFileVersionMetadata{
		FileId:    fileID,
		FileName:  header.Filename,
		SizeBytes: header.Size,
	}

	ctx, cancel := context.WithTimeout(r.Context(), uploadTimeout)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeader)

	resp, err := u.streamVersion(ctx, meta, file)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	u.writeProtoResponse(w, r, http.StatusOK, resp)
}

func (u *uploadHandler) parseMultipartFile(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, func(), bool) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(uploadTimeout))
	_ = rc.SetWriteDeadline(time.Now().Add(uploadTimeout))

	r.Body = http.MaxBytesReader(w, r.Body, u.maxBytes+multipartOverhead)
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			writeError(w, http.StatusRequestEntityTooLarge, codes.InvalidArgument,
				fmt.Sprintf("File is too large: the maximum size of a single file is %s", humanSize(u.maxBytes)))
		case errors.Is(err, http.ErrNotMultipart):
			writeError(w, http.StatusBadRequest, codes.InvalidArgument, "Upload must be sent as multipart/form-data")
		default:
			writeError(w, http.StatusBadRequest, codes.InvalidArgument, "Invalid multipart upload request")
		}
		return nil, nil, nil, false
	}

	cleanup := func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		cleanup()
		writeError(w, http.StatusBadRequest, codes.InvalidArgument, "Please select a file to upload")
		return nil, nil, nil, false
	}

	return file, header, cleanup, true
}

func (u *uploadHandler) writeProtoResponse(w http.ResponseWriter, r *http.Request, statusCode int, resp *storagepb.UploadFileResponse) {
	_, outbound := runtime.MarshalerForRequest(u.gw, r)
	body, err := outbound.Marshal(resp)
	if err != nil {
		log.Printf("marshal upload response: %v", err)
		writeError(w, http.StatusInternalServerError, codes.Internal, "Internal error")
		return
	}
	w.Header().Set("Content-Type", outbound.ContentType(resp))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

// stream sends the metadata and then the content of file to storage-service.
func (u *uploadHandler) stream(ctx context.Context, meta *storagepb.UploadFileMetadata, file multipart.File) (*storagepb.UploadFileResponse, error) {
	stream, err := u.storage.UploadFile(ctx)
	if err != nil {
		return nil, err
	}

	send := func(m *storagepb.UploadFileRequest) bool { return stream.Send(m) == nil }

	if send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Metadata{Metadata: meta}}) {
		buf := make([]byte, uploadChunkSize)
		for {
			n, readErr := file.Read(buf)
			if n > 0 {
				chunk := &storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Chunk{Chunk: buf[:n]}}
				if !send(chunk) {
					break // storage-service ended the call early; CloseAndRecv below returns its status
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				log.Printf("read spooled upload: %v", readErr)
				return nil, status.Error(codes.Internal, "Failed to read the uploaded file")
			}
		}
	}

	// A failed Send only says "the stream is over"; the real reason (quota
	// exceeded, bad file type, ...) is the status that CloseAndRecv returns.
	return stream.CloseAndRecv()
}

func (u *uploadHandler) streamVersion(ctx context.Context, meta *storagepb.UploadNewFileVersionMetadata, file multipart.File) (*storagepb.UploadFileResponse, error) {
	stream, err := u.storage.UploadNewFileVersion(ctx)
	if err != nil {
		return nil, err
	}

	send := func(m *storagepb.UploadNewFileVersionRequest) bool { return stream.Send(m) == nil }

	if send(&storagepb.UploadNewFileVersionRequest{Payload: &storagepb.UploadNewFileVersionRequest_Metadata{Metadata: meta}}) {
		buf := make([]byte, uploadChunkSize)
		for {
			n, readErr := file.Read(buf)
			if n > 0 {
				chunk := &storagepb.UploadNewFileVersionRequest{Payload: &storagepb.UploadNewFileVersionRequest_Chunk{Chunk: buf[:n]}}
				if !send(chunk) {
					break
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				log.Printf("read spooled version upload: %v", readErr)
				return nil, status.Error(codes.Internal, "Failed to read the uploaded file")
			}
		}
	}

	return stream.CloseAndRecv()
}

func (u *uploadHandler) validToken(authHeader string) bool {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	token, err := jwt.Parse(strings.TrimPrefix(authHeader, "Bearer "), func(*jwt.Token) (any, error) {
		return u.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	return err == nil && token.Valid
}

// writeGRPCError maps a gRPC status to HTTP exactly like grpc-gateway does for
// the generated routes, with one deliberate exception: ResourceExhausted from
// the upload means "storage quota exceeded" and is answered with 413 Payload
// Too Large, not with 429, which the frontend (and the rate limiter) use for
// "too many requests".
func writeGRPCError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	httpCode := runtime.HTTPStatusFromCode(st.Code())
	if st.Code() == codes.ResourceExhausted {
		httpCode = http.StatusRequestEntityTooLarge
	}
	writeError(w, httpCode, st.Code(), st.Message())
}

// writeError writes the same body shape grpc-gateway uses for errors:
// {"code": <grpc code>, "message": "...", "details": []}.
func writeError(w http.ResponseWriter, httpCode int, code codes.Code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    int(code),
		"message": message,
		"details": []any{},
	})
}

// humanSize renders 1572864 as "1.5 MB".
func humanSize(n int64) string {
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
