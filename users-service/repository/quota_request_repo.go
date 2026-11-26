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

// ErrPendingQuotaRequestExists: the user already has a request awaiting a decision.
var ErrPendingQuotaRequestExists = errors.New("a pending quota request already exists")

type QuotaRequestRepository struct {
	collection *mongo.Collection
}

// NewQuotaRequestRepository creates the indexes, giving up after setupTimeout.
func NewQuotaRequestRepository(db *mongo.Database, setupTimeout time.Duration) (*QuotaRequestRepository, error) {
	coll := db.Collection("quota_requests")

	models := []mongo.IndexModel{
		// At most one pending request per user. The database enforces it, so two
		// simultaneous submissions cannot both get through.
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

	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return nil, err
	}
	return &QuotaRequestRepository{collection: coll}, nil
}

// Create stores a new request and sets its ID.
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

// FindAll returns the requests with the given status, or all of them when
// status is empty, newest first.
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

// Resolve moves a PENDING request to status in one atomic update, so a request
// can be decided only once. It returns mongo.ErrNoDocuments when there is no
// pending request with this ID.
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

// Reopen puts a request back to pending after applying its approval failed.
func (r *QuotaRequestRepository) Reopen(ctx context.Context, id primitive.ObjectID) error {
	update := bson.M{
		"$set":   bson.M{"status": domain.QuotaRequestPending},
		"$unset": bson.M{"resolvedAt": "", "resolvedBy": ""},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
