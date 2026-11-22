package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"storage-service/domain"
	"storage-service/middleware"
	"storage-service/repository"
	"storage-service/service"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileHandler struct {
	fileRepo    *repository.FileRepository
	folderRepo  *repository.FolderRepository
	objectStore *service.ObjectStore
	quotaClient *service.QuotaClient
}

func NewFileHandler(
	fileRepo *repository.FileRepository,
	folderRepo *repository.FolderRepository,
	objectStore *service.ObjectStore,
	quotaClient *service.QuotaClient,
) *FileHandler {
	return &FileHandler{
		fileRepo:    fileRepo,
		folderRepo:  folderRepo,
		objectStore: objectStore,
		quotaClient: quotaClient,
	}
}

// POST /api/files/upload (multipart/form-data: file, folderId)
func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := r.Context().Value(middleware.UserIDKey).(string)

	// Parsiraj multipart formu (do 32 MB u memoriji, ostatak u privremenom fajlu na disku)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid multipart upload request"})
		return
	}

	uploadedFile, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Please select a file to upload"})
		return
	}
	defer uploadedFile.Close()

	cleanName := strings.TrimSpace(filepath.Base(header.Filename))
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.ContainsAny(cleanName, `/\:*?"<>|`) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid file name"})
		return
	}

	fileSize := header.Size
	if fileSize <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Cannot upload an empty file (0 bytes)"})
		return
	}

	// Provera ciljnog foldera (ako nije root)
	var folderObjID *primitive.ObjectID
	folderIDStr := strings.TrimSpace(r.FormValue("folderId"))
	if folderIDStr != "" {
		oid, err := primitive.ObjectIDFromHex(folderIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Invalid target folder ID"})
			return
		}

		folder, err := h.folderRepo.FindByID(r.Context(), oid)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeJSON(w, http.StatusNotFound, map[string]string{"message": "Target folder not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error verifying target folder"})
			return
		}

		if folder.OwnerID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "Access denied to target folder"})
			return
		}

		folderObjID = &oid
	}

	// Provera da li fajl istog naziva već postoji u istom direktorijumu
	exists, err := h.fileRepo.ExistsByNameInFolder(r.Context(), userID, folderObjID, cleanName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Error checking file uniqueness"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"message": "A file with this name already exists in the current folder"})
		return
	}

	// AC 2.2.2: Sinhrona provera kvote sa Users servisom PRE snimanja fajla
	quotaResp, statusCode, err := h.quotaClient.ReserveQuota(r.Context(), userID, fileSize)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"message": "Unable to verify storage quota with Users service"})
		return
	}

	// AC 2.2.4: Ako je kvota prekoračena, odmah prekini upload sa jasnom porukom
	if statusCode != http.StatusOK || !quotaResp.Allowed {
		msg := quotaResp.Message
		if msg == "" {
			msg = "Storage quota exceeded. Upload aborted."
		}
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]interface{}{
			"message":           msg,
			"storageQuotaBytes": quotaResp.StorageQuotaBytes,
			"usedStorageBytes":  quotaResp.UsedStorageBytes,
			"remainingBytes":    quotaResp.RemainingBytes,
		})
		return
	}

	// AC 2.2.3: Snimanje binarnog sadržaja u Object Store (MinIO)
	fileID := primitive.NewObjectID()
	objectKey := fmt.Sprintf("%s/%s_%s", userID, fileID.Hex(), cleanName)
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	if err := h.objectStore.UploadObject(r.Context(), objectKey, uploadedFile, fileSize, mimeType); err != nil {
		h.quotaClient.ReleaseQuota(r.Context(), userID, fileSize)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to store file content in Object Store"})
		return
	}

	// AC 2.2.3: Snimanje metapodataka u document-oriented bazu (MongoDB)
	fileDoc := &domain.FileItem{
		ID:        fileID,
		Name:      cleanName,
		FolderID:  folderObjID,
		OwnerID:   userID,
		SizeBytes: fileSize,
		MimeType:  mimeType,
		ObjectKey: objectKey,
		CreatedAt: time.Now(),
	}

	if err := h.fileRepo.Create(r.Context(), fileDoc); err != nil {
		_ = h.objectStore.DeleteObject(r.Context(), objectKey)
		h.quotaClient.ReleaseQuota(r.Context(), userID, fileSize)

		if mongo.IsDuplicateKeyError(err) {
			writeJSON(w, http.StatusConflict, map[string]string{"message": "A file with this name already exists in the current folder"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Failed to save file metadata"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":           "File uploaded successfully",
		"file":              fileDoc,
		"storageQuotaBytes": quotaResp.StorageQuotaBytes,
		"usedStorageBytes":  quotaResp.UsedStorageBytes,
		"remainingBytes":    quotaResp.RemainingBytes,
	})
}

// GET /api/files/quota
func (h *FileHandler) GetMyQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := r.Context().Value(middleware.UserIDKey).(string)
	quotaResp, statusCode, err := h.quotaClient.GetQuota(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"message": "Unable to retrieve storage quota"})
		return
	}
	if statusCode != http.StatusOK {
		writeJSON(w, statusCode, map[string]string{"message": quotaResp.Message})
		return
	}

	writeJSON(w, http.StatusOK, quotaResp)
}