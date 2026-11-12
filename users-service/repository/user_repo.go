package repository

import (
	"context"
	"time"
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
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) SaveOTP(ctx context.Context, email string, otp string, expiresAt time.Time) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"otp":         otp,
			"otpExpires":  expiresAt,
			"otpAttempts": 0,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) IncrementOTPAttempts(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$inc": bson.M{"otpAttempts": 1},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) ClearOTP(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$unset": bson.M{"otp": "", "otpExpires": ""},
		"$set":   bson.M{"otpAttempts": 0},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) SavePasswordResetToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"passwordResetTokenHash":    tokenHash,
			"passwordResetTokenExpires": expiresAt,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) FindByPasswordResetTokenHash(ctx context.Context, tokenHash string) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{"passwordResetTokenHash": tokenHash}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ClearPasswordResetToken(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$unset": bson.M{"passwordResetTokenHash": "", "passwordResetTokenExpires": ""},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, email, newHash string, changedAt time.Time) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"passwordHash":        newHash,
			"passwordLastChanged": changedAt,
		},
		"$unset": bson.M{
			"passwordResetTokenHash":    "",
			"passwordResetTokenExpires": "",
			"accountLockedUntil":        "",
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) IncrementResetRequestCount(ctx context.Context, email string, windowStart time.Time, resetWindow bool) error {
	filter := bson.M{"email": email}
	var update bson.M
	if resetWindow {
		update = bson.M{
			"$set": bson.M{
				"passwordResetWindowStart":  windowStart,
				"passwordResetRequestCount": 1,
			},
		}
	} else {
		update = bson.M{
			"$inc": bson.M{"passwordResetRequestCount": 1},
		}
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) LockAccount(ctx context.Context, email string, until time.Time) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"accountLockedUntil": until,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) UnlockAccount(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$unset": bson.M{
			"accountLockedUntil": "",
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) SaveMagicLinkToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"magicLinkTokenHash":    tokenHash,
			"magicLinkTokenExpires": expiresAt,
		},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *UserRepository) FindByMagicLinkToken(ctx context.Context, tokenHash string) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{"magicLinkTokenHash": tokenHash}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ClearMagicLinkToken(ctx context.Context, email string) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$unset": bson.M{"magicLinkTokenHash": "", "magicLinkTokenExpires": ""},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err

}
