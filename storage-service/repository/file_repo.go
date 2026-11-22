package repository

import (
	"context"
	"storage-service/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FileRepository struct {
	collection *mongo.Collection
}

func NewFileRepository(db *mongo.Database) *FileRepository {
	col := db.Collection("files")

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerId", Value: 1},
			{Key: "folderId", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, _ = col.Indexes().CreateOne(context.Background(), indexModel)

	return &FileRepository{collection: col}
}

func (r *FileRepository) Create(ctx context.Context, file *domain.FileItem) error {
	res, err := r.collection.InsertOne(ctx, file)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		file.ID = oid
	}
	return nil
}

func (r *FileRepository) ExistsByNameInFolder(ctx context.Context, ownerID string, folderID *primitive.ObjectID, name string) (bool, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"folderId": folderID,
		"name":     name,
	}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *FileRepository) FindByFolder(ctx context.Context, ownerID string, folderID *primitive.ObjectID) ([]domain.FileItem, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"folderId": folderID,
	}
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	files := make([]domain.FileItem, 0)
	if err := cursor.All(ctx, &files); err != nil {
		return nil, err
	}
	return files, nil
}