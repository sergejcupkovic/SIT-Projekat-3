package repository

import (
	"context"
	"storage-service/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FolderRepository struct {
	collection *mongo.Collection
}

func NewFolderRepository(db *mongo.Database) *FolderRepository {
	col := db.Collection("folders")

	// Sprečava duplikate naziva foldera unutar istog roditeljskog foldera za istog korisnika
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "ownerId", Value: 1},
			{Key: "parentId", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, _ = col.Indexes().CreateOne(context.Background(), indexModel)

	return &FolderRepository{collection: col}
}

func (r *FolderRepository) Create(ctx context.Context, folder *domain.Folder) error {
	res, err := r.collection.InsertOne(ctx, folder)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		folder.ID = oid
	}
	return nil
}

func (r *FolderRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Folder, error) {
	var folder domain.Folder
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&folder)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func (r *FolderRepository) ExistsByNameInParent(ctx context.Context, ownerID string, parentID *primitive.ObjectID, name string) (bool, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"parentId": parentID,
		"name":     name,
	}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *FolderRepository) FindChildren(ctx context.Context, ownerID string, parentID *primitive.ObjectID) ([]domain.Folder, error) {
	filter := bson.M{
		"ownerId":  ownerID,
		"parentId": parentID,
	}
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	folders := make([]domain.Folder, 0)
	if err := cursor.All(ctx, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *FolderRepository) UpdateName(ctx context.Context, id primitive.ObjectID, newName string) (*domain.Folder, error) {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"name": newName}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updated domain.Folder
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}