package repository

import (
	"context"
	"users-service/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// za interaction sa mongodb
type UserRepository struct {
	collection *mongo.Collection
}

// returnuje pointer na novi userrepo ili error
func NewUserRepository(db *mongo.Database) (*UserRepository, error) {
	coll := db.Collection("users")

	//kreiranje unikatnog indexa na email polju
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}}, //bson.d kaze da indeksira rastuce
		Options: options.Index().SetUnique(true),
	}
	_, err := coll.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		return nil, err
	}

	return &UserRepository{collection: coll}, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

// verifikacija usera u bazi, postavi flag isverified na true i obrise token da ne moze ponovo da se iskoristi
// bson.M je za unordered map, a bson.D je za ordered map
// $set je promena $unset kompletno obrise polje jer nije vise needed
func (r *UserRepository) VerifyToken(ctx context.Context, token string) (*mongo.UpdateResult, error) {
	filter := bson.M{"verificationToken": token}
	update := bson.M{
		"$set":   bson.M{"isVerified": true},
		"$unset": bson.M{"verificationToken": "", "verificationTokenExpires": ""},
	}
	return r.collection.UpdateOne(ctx, filter, update)
}
