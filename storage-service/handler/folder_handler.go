package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"storage-service/domain"
	"storage-service/middleware"
	"storage-service/repository"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FolderHandler struct {
	repo *repository.FolderRepository
	fileRepo *repository.FileRepository
}

func NewFolderHandler(repo *repository.FolderRepository, fileRepo *repository.FileRepository) *FolderHandler {
	return &FolderHandler{repo: repo, fileRepo: fileRepo}
}

type CreateFolderRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

type RenameFolderRequest struct {
	Name string `json:"name"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func validateFolderName(name string) error {
	if name == "" {
		return errors.New("folder name is required")
	}
	if len(name) > 100 {
		return errors.New("folder name must not exceed 100 characters")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\:*?"<>|`) {
		return errors.New("folder name contains invalid characters")
	}
	return nil
}

// HandleFolders rukuje sa POST /api/folders i GET /api/folders?parentId=...
func (h *FolderHandler) HandleFolders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateFolder(w, r)
	case http.MethodGet:
		parentID := r.URL.Query().Get("parentId")
		h.BrowseFolder(w, r, parentID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleFolderByID rukuje sa GET, PATCH i PUT /api/folders/{id}
func (h *FolderHandler) HandleFolderByID(w http.ResponseWriter, r *http.Request) {
	folderID := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	folderID = strings.TrimSuffix(folderID, "/")

	switch r.Method {
	case http.MethodGet:
		h.BrowseFolder(w, r, folderID)
	case http.MethodPatch, http.MethodPut:
		h.RenameFolder(w, r, folderID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *FolderHandler) RenameFolder(w http.ResponseWriter, r *http.Request, folderID string) {
	userID, _ := r.Context().Value(middleware.UserIDKey).(string)

	oid, err := primitive.ObjectIDFromHex(folderID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid folder ID"})
		return
	}

	var req RenameFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid JSON request body"})
		return
	}

	cleanName := strings.TrimSpace(req.Name)
	if err := validateFolderName(cleanName); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}

	folder, err := h.repo.FindByID(r.Context(), oid)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "Folder not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error loading folder"})
		return
	}

	if folder.OwnerID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"message": "You do not have permission to rename this folder"})
		return
	}

	if folder.Name == cleanName {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Folder renamed successfully",
			"folder":  folder,
		})
		return
	}

	exists, err := h.repo.ExistsByNameInParent(r.Context(), userID, folder.ParentID, cleanName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error checking folder uniqueness"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"message": "A folder with this name already exists in the current directory"})
		return
	}

	updatedFolder, err := h.repo.UpdateName(r.Context(), oid, cleanName)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "A folder with this name already exists in the current directory"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to rename folder"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Folder renamed successfully",
		"folder":  updatedFolder,
	})
}

func (h *FolderHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.UserIDKey).(string)

	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid JSON request body"})
		return
	}

	cleanName := strings.TrimSpace(req.Name)
	if err := validateFolderName(cleanName); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return
	}

	var parentObjID *primitive.ObjectID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(*req.ParentID))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid parent folder ID"})
			return
		}

		parentFolder, err := h.repo.FindByID(r.Context(), oid)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				writeJSON(w, http.StatusNotFound, map[string]string{"message": "Parent folder not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error verifying parent folder"})
			return
		}

		if parentFolder.OwnerID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "You do not have permission to access this parent folder"})
			return
		}

		parentObjID = &oid
	}

	exists, err := h.repo.ExistsByNameInParent(r.Context(), userID, parentObjID, cleanName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error checking folder uniqueness"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"message": "A folder with this name already exists in the current directory"})
		return
	}

	folder := &domain.Folder{
		Name:      cleanName,
		ParentID:  parentObjID,
		OwnerID:   userID,
		CreatedAt: time.Now(),
	}

	if err := h.repo.Create(r.Context(), folder); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "A folder with this name already exists in the current directory"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to create folder"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Folder created successfully",
		"folder":  folder,
	})
}

func (h *FolderHandler) BrowseFolder(w http.ResponseWriter, r *http.Request, folderID string) {
	userID, _ := r.Context().Value(middleware.UserIDKey).(string)

	var currentFolder *domain.Folder
	var parentObjID *primitive.ObjectID
	breadcrumbs := make([]domain.BreadcrumbItem, 0)

	if folderID != "" {
		oid, err := primitive.ObjectIDFromHex(folderID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid folder ID"})
			return
		}

		folder, err := h.repo.FindByID(r.Context(), oid)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				writeJSON(w, http.StatusNotFound, map[string]string{"message": "Folder not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error loading folder"})
			return
		}

		if folder.OwnerID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Access denied to this folder"})
			return
		}

		currentFolder = folder
		parentObjID = &oid

		crumbs, err := h.buildBreadcrumbs(r.Context(), folder, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error building folder hierarchy path"})
			return
		}
		breadcrumbs = crumbs
	}

	subfolders, err := h.repo.FindChildren(r.Context(), userID, parentObjID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error retrieving folder contents"})
		return
	}

	files, err := h.fileRepo.FindByFolder(r.Context(), userID, parentObjID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error retrieving files"})
		return
	}

	resp := domain.FolderContentResponse{
		CurrentFolder: currentFolder,
		Breadcrumbs:   breadcrumbs,
		Folders:       subfolders,
		Files:         files,
	}

	writeJSON(w, http.StatusOK, resp)
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