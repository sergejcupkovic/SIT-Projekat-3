// Package handler is the transport layer of storage-service.
//
// FolderHandler implements storagepb.StorageServiceServer, generated from
// proto/storage/storage_service.proto. Authentication has already been done
// by middleware.AuthUnaryInterceptor, which put the caller's user ID in ctx.
// Errors are gRPC status codes; the api-gateway maps them back to HTTP
// (InvalidArgument 400, NotFound 404, PermissionDenied 403, AlreadyExists 409,
// Internal 500).
package handler

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"storage-service/domain"
	"storage-service/middleware"
	"storage-service/repository"

	storagepb "stashr/proto/storage"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FolderHandler struct {
	storagepb.UnimplementedStorageServiceServer

	repo *repository.FolderRepository
}

var _ storagepb.StorageServiceServer = (*FolderHandler)(nil)

func NewFolderHandler(repo *repository.FolderRepository) *FolderHandler {
	return &FolderHandler{repo: repo}
}

const duplicateNameMsg = "A folder with this name already exists in the current directory"

// RenameFolder  <-  PATCH|PUT /api/folders/{folder_id}  body {"name": "..."}
func (h *FolderHandler) RenameFolder(ctx context.Context, req *storagepb.RenameFolderRequest) (*storagepb.FolderResponse, error) {
	userID := userIDFrom(ctx)

	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(req.GetFolderId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid folder ID")
	}

	cleanName := strings.TrimSpace(req.GetName())
	if err := validateFolderName(cleanName); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	folder, err := h.repo.FindByID(ctx, oid)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, "Folder not found")
		}
		return nil, status.Error(codes.Internal, "Error loading folder")
	}

	if folder.OwnerID != userID {
		return nil, status.Error(codes.PermissionDenied, "You do not have permission to rename this folder")
	}

	if folder.Name == cleanName {
		return &storagepb.FolderResponse{
			Message: "Folder renamed successfully",
			Folder:  toProtoFolder(folder),
		}, nil
	}

	exists, err := h.repo.ExistsByNameInParent(ctx, userID, folder.ParentID, cleanName)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error checking folder uniqueness")
	}
	if exists {
		return nil, status.Error(codes.AlreadyExists, duplicateNameMsg)
	}

	updatedFolder, err := h.repo.UpdateName(ctx, oid, cleanName)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, status.Error(codes.AlreadyExists, duplicateNameMsg)
		}
		return nil, status.Error(codes.Internal, "Failed to rename folder")
	}

	return &storagepb.FolderResponse{
		Message: "Folder renamed successfully",
		Folder:  toProtoFolder(updatedFolder),
	}, nil
}

// CreateFolder  <-  POST /api/folders  body {"name": "...", "parentId": "..."|null}  (HTTP 201)
func (h *FolderHandler) CreateFolder(ctx context.Context, req *storagepb.CreateFolderRequest) (*storagepb.FolderResponse, error) {
	userID := userIDFrom(ctx)

	cleanName := strings.TrimSpace(req.GetName())
	if err := validateFolderName(cleanName); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var parentObjID *primitive.ObjectID
	if req.ParentId != nil && strings.TrimSpace(req.GetParentId()) != "" {
		oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(req.GetParentId()))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Invalid parent folder ID")
		}

		parentFolder, err := h.repo.FindByID(ctx, oid)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, status.Error(codes.NotFound, "Parent folder not found")
			}
			return nil, status.Error(codes.Internal, "Error verifying parent folder")
		}

		if parentFolder.OwnerID != userID {
			return nil, status.Error(codes.PermissionDenied, "You do not have permission to access this parent folder")
		}

		parentObjID = &oid
	}

	exists, err := h.repo.ExistsByNameInParent(ctx, userID, parentObjID, cleanName)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error checking folder uniqueness")
	}
	if exists {
		return nil, status.Error(codes.AlreadyExists, duplicateNameMsg)
	}

	folder := &domain.Folder{
		Name:      cleanName,
		ParentID:  parentObjID,
		OwnerID:   userID,
		CreatedAt: time.Now(),
	}

	if err := h.repo.Create(ctx, folder); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, status.Error(codes.AlreadyExists, duplicateNameMsg)
		}
		return nil, status.Error(codes.Internal, "Failed to create folder")
	}

	setHTTPStatus(ctx, 201)
	return &storagepb.FolderResponse{
		Message: "Folder created successfully",
		Folder:  toProtoFolder(folder),
	}, nil
}

// BrowseFolder  <-  GET /api/folders/{folder_id}  or  GET /api/folders[?parentId=...]
func (h *FolderHandler) BrowseFolder(ctx context.Context, req *storagepb.BrowseFolderRequest) (*storagepb.FolderContentResponse, error) {
	userID := userIDFrom(ctx)

	folderID := strings.TrimSpace(req.GetFolderId())
	if folderID == "" {
		folderID = strings.TrimSpace(req.GetParentId())
	}

	var currentFolder *domain.Folder
	var parentObjID *primitive.ObjectID
	breadcrumbs := make([]domain.BreadcrumbItem, 0)

	if folderID != "" {
		oid, err := primitive.ObjectIDFromHex(folderID)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Invalid folder ID")
		}

		folder, err := h.repo.FindByID(ctx, oid)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, status.Error(codes.NotFound, "Folder not found")
			}
			return nil, status.Error(codes.Internal, "Error loading folder")
		}

		if folder.OwnerID != userID {
			return nil, status.Error(codes.PermissionDenied, "Access denied to this folder")
		}

		currentFolder = folder
		parentObjID = &oid

		crumbs, err := h.buildBreadcrumbs(ctx, folder, userID)
		if err != nil {
			return nil, status.Error(codes.Internal, "Error building folder hierarchy path")
		}
		breadcrumbs = crumbs
	}

	subfolders, err := h.repo.FindChildren(ctx, userID, parentObjID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Error retrieving folder contents")
	}

	resp := &storagepb.FolderContentResponse{
		Breadcrumbs: make([]*storagepb.BreadcrumbItem, 0, len(breadcrumbs)),
		Folders:     make([]*storagepb.Folder, 0, len(subfolders)),
		Files:       make([]*storagepb.FileItem, 0),
	}
	if currentFolder != nil {
		resp.CurrentFolder = toProtoFolder(currentFolder)
	}
	for _, c := range breadcrumbs {
		resp.Breadcrumbs = append(resp.Breadcrumbs, &storagepb.BreadcrumbItem{Id: c.ID, Name: c.Name})
	}
	for i := range subfolders {
		resp.Folders = append(resp.Folders, toProtoFolder(&subfolders[i]))
	}

	return resp, nil
}

func (h *FolderHandler) buildBreadcrumbs(ctx context.Context, current *domain.Folder, ownerID string) ([]domain.BreadcrumbItem, error) {
	var reversed []domain.BreadcrumbItem
	visited := make(map[string]bool)

	curr := current
	for curr != nil {
		idHex := curr.ID.Hex()
		if visited[idHex] {
			break
		}
		visited[idHex] = true

		reversed = append(reversed, domain.BreadcrumbItem{
			ID:   idHex,
			Name: curr.Name,
		})

		if curr.ParentID == nil {
			break
		}

		parent, err := h.repo.FindByID(ctx, *curr.ParentID)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				break
			}
			return nil, err
		}
		if parent.OwnerID != ownerID {
			break
		}
		curr = parent
	}

	// Obrćemo niz da ide od najvišeg foldera ka trenutnom
	breadcrumbs := make([]domain.BreadcrumbItem, len(reversed))
	for i := range reversed {
		breadcrumbs[i] = reversed[len(reversed)-1-i]
	}
	return breadcrumbs, nil
}

// #region helperi

const maxFolderNameLen = 100

// letters, digits, space and . _ - ( ); everything else is rejected
var folderNameRegex = regexp.MustCompile(`^[\p{L}\p{N} ._()\-]+$`)

func validateFolderName(name string) error {
	if name == "" {
		return errors.New("folder name is required")
	}
	if utf8.RuneCountInString(name) > maxFolderNameLen {
		return errors.New("folder name must not exceed 100 characters")
	}
	if !folderNameRegex.MatchString(name) || strings.Trim(name, ". ") == "" {
		return errors.New("folder name contains invalid characters")
	}
	return nil
}

// toProtoFolder maps the MongoDB model to the protobuf message.
func toProtoFolder(f *domain.Folder) *storagepb.Folder {
	pf := &storagepb.Folder{
		Id:        f.ID.Hex(),
		Name:      f.Name,
		OwnerId:   f.OwnerID,
		CreatedAt: timestamppb.New(f.CreatedAt),
	}
	if f.ParentID != nil {
		parent := f.ParentID.Hex()
		pf.ParentId = &parent
	}
	return pf
}

func userIDFrom(ctx context.Context) string {
	userID, _ := ctx.Value(middleware.UserIDKey).(string)
	return userID
}

// setHTTPStatus sends response-header metadata "x-http-code" that the
// api-gateway turns into the HTTP status code (e.g. 201 Created).
func setHTTPStatus(ctx context.Context, code int) {
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-http-code", strconv.Itoa(code))); err != nil {
		log.Printf("failed to set x-http-code header: %v", err)
	}
}

// #endregion
