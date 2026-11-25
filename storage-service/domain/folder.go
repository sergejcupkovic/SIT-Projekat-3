package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Folder struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name      string              `bson:"name" json:"name"`
	ParentID  *primitive.ObjectID `bson:"parentId" json:"parentId"`
	OwnerID   string              `bson:"ownerId" json:"ownerId"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
}

type BreadcrumbItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FileItem is the metadata document of an uploaded file (collection "files").
// The content itself lives in the object store under ObjectKey. FolderID is nil
// for files in the root ("My Drive").
type FileItem struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name      string              `bson:"name" json:"name"`
	FolderID  *primitive.ObjectID `bson:"folderId" json:"folderId"`
	OwnerID   string              `bson:"ownerId" json:"ownerId"`
	SizeBytes int64               `bson:"sizeBytes" json:"sizeBytes"`
	MimeType  string              `bson:"mimeType" json:"mimeType"`
	ObjectKey string              `bson:"objectKey" json:"-"`
	// SHA-256 (hex) of the stored content, for the integrity check on download.
	SHA256    string    `bson:"sha256" json:"-"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson: "updatedAt,omitempty"`
}

type FolderContentResponse struct {
	CurrentFolder *Folder          `json:"currentFolder"`
	Breadcrumbs   []BreadcrumbItem `json:"breadcrumbs"`
	Folders       []Folder         `json:"folders"`
	Files         []FileItem       `json:"files"`
}