package repository

import (
	"context"
	"errors"
	"time"
	"users-service/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrPendingQuotaRequestExists = errors.New("a pending quota request already exists")

type QuotaRequestRepository struct {
	collection *mongo.Collection
}

func NewQuotaRequestRepository(db *mongo.Database) (*QuotaRequestRepository, error) {
	coll := db.Collection("quota_requests")

	models := []mongo.IndexModel{
		// the database allows only one pending request per user
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().
				SetName("one_pending_per_user").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"status": domain.QuotaRequestPending}),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return nil, err
	}
	return &QuotaRequestRepository{collection: coll}, nil
}

func (r *QuotaRequestRepository) Create(ctx context.Context, request *domain.QuotaRequest) error {
	res, err := r.collection.InsertOne(ctx, request)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrPendingQuotaRequestExists
		}
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		request.ID = oid
	}
	return nil
}

func (r *QuotaRequestRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.QuotaRequest, error) {
	var request domain.QuotaRequest
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&request); err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *QuotaRequestRepository) FindByUser(ctx context.Context, userID primitive.ObjectID) ([]domain.QuotaRequest, error) {
	return r.find(ctx, bson.M{"userId": userID})
}

func (r *QuotaRequestRepository) FindAll(ctx context.Context, status domain.QuotaRequestStatus) ([]domain.QuotaRequest, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	return r.find(ctx, filter)
}

func (r *QuotaRequestRepository) find(ctx context.Context, filter bson.M) ([]domain.QuotaRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var requests []domain.QuotaRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}
	return requests, nil
}

func (r *QuotaRequestRepository) Resolve(ctx context.Context, id primitive.ObjectID, status domain.QuotaRequestStatus, adminID string, at time.Time) (*domain.QuotaRequest, error) {
	filter := bson.M{"_id": id, "status": domain.QuotaRequestPending}
	update := bson.M{"$set": bson.M{"status": status, "resolvedAt": at, "resolvedBy": adminID}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updated domain.QuotaRequest
	if err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated); err != nil {
		return nil, err
	}
	return &updated, nil
}
