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

type FileItem struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name      string              `bson:"name" json:"name"`
	FolderID  *primitive.ObjectID `bson:"folderId" json:"folderId"`
	OwnerID   string              `bson:"ownerId" json:"ownerId"`
	SizeBytes int64               `bson:"sizeBytes" json:"sizeBytes"`
	MimeType  string              `bson:"mimeType" json:"mimeType"`
	ObjectKey string              `bson:"objectKey" json:"-"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
}

type FolderContentResponse struct {
	CurrentFolder *Folder          `json:"currentFolder"`
	Breadcrumbs   []BreadcrumbItem `json:"breadcrumbs"`
	Folders       []Folder         `json:"folders"`
	Files         []FileItem       `json:"files"`
}