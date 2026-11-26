package repository

import (
	"time"
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

	// No two files with the same name in the same folder of the same user.
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

func (r *FileRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.FileItem, error) {
	var file domain.FileItem
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) UpdateVersion(ctx context.Context, file *domain.FileItem) error {
	filter := bson.M{
		"_id":     file.ID,
		"ownerId": file.OwnerID,
	}
	update := bson.M{
		"$set": bson.M{
			"sizeBytes": file.SizeBytes,
			"mimeType":  file.MimeType,
			"objectKey": file.ObjectKey,
			"sha256":    file.SHA256,
			"updatedAt": file.UpdatedAt,
		},
	}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *FileRepository) Rename(ctx context.Context, id primitive.ObjectID, ownerID, newName string, updatedAt time.Time) error {
	filter := bson.M{
		"_id":     id,
		"ownerId": ownerID,
	}
	update := bson.M{
		"$set": bson.M{
			"name":      newName,
			"updatedAt": updatedAt,
		},
	}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}