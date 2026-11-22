package repository

import (
	"context"
	"time"
	"users-service/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// za interaction sa mongodb
type UserRepository struct {
	collection *mongo.Collection
}

// returnuje pointer na novi userrepo ili error
func NewUserRepository(db *mongo.Database) (*UserRepository, error) {
	coll := db.Collection("users")

	// indeksi: unikatan email + sparse indeksi za tokene (da upiti ne rade full scan)
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "verificationToken", Value: 1}}, Options: options.Index().SetSparse(true)},
		{Keys: bson.D{{Key: "passwordResetTokenHash", Value: 1}}, Options: options.Index().SetSparse(true)},
		{Keys: bson.D{{Key: "magicLinkTokenHash", Value: 1}}, Options: options.Index().SetSparse(true)},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return nil, err
	}

	return &UserRepository{collection: coll}, nil
}

// #region account (create / lookup / verify)

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

// verifikacija usera u bazi, postavi flag isverified na true i obrise token da ne moze ponovo da se iskoristi
// bson.M je za unordered map, a bson.D je za ordered map
// $set je promena $unset kompletno obrise polje jer nije vise needed
func (r *UserRepository) VerifyToken(ctx context.Context, token string) (*mongo.UpdateResult, error) {
	filter := bson.M{
		"verificationToken":        token,
		"verificationTokenExpires": bson.M{"$gt": time.Now()},
	}
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

// #endregion

// #region OTP
func (r *UserRepository) SaveOTP(ctx context.Context, email string, otp string, expiresAt time.Time, resetAttempts bool) error {
	filter := bson.M{"email": email}
	set := bson.M{
		"otp":        otp,
		"otpExpires": expiresAt,
	}
	if resetAttempts {
		set["otpAttempts"] = 0
	}
	_, err := r.collection.UpdateOne(ctx, filter, bson.M{"$set": set})
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

// #endregion

// #region Password Reset
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
	return r.IncrementWindowedCount(ctx, email, "passwordResetRequestCount", "passwordResetWindowStart", windowStart, resetWindow)
}

func (r *UserRepository) IncrementWindowedCount(ctx context.Context, email, countField, windowField string, windowStart time.Time, resetWindow bool) error {
	filter := bson.M{"email": email}
	var update bson.M
	if resetWindow {
		update = bson.M{
			"$set": bson.M{
				windowField: windowStart,
				countField:  1,
			},
		}
	} else {
		update = bson.M{
			"$inc": bson.M{countField: 1},
		}
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

// #endregion

// #region account lock
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

// #endregion

// #region magic link
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
	err := r.collection.FindOne(ctx, bson.M{"magicLinkTokenHash": tokenHash, "magicLinkTokenExpires": bson.M{"$gt": time.Now()}}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ConsumeMagicLinkToken(ctx context.Context, tokenHash string) (*domain.User, error) {
	filter := bson.M{
		"magicLinkTokenHash":    tokenHash,
		"magicLinkTokenExpires": bson.M{"$gt": time.Now()},
	}
	update := bson.M{
		"$unset": bson.M{"magicLinkTokenHash": "", "magicLinkTokenExpires": ""},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)
	var user domain.User
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
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



// #endregion
// #region Storage Quota
func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ReserveStorageQuota atomski proverava da li korisnik ima dovoljno slobodnog prostora i rezerviše ga.
func (r *UserRepository) ReserveStorageQuota(ctx context.Context, userID primitive.ObjectID, sizeBytes int64) (*domain.User, bool, error) {
	filter := bson.M{
		"_id": userID,
		"$expr": bson.M{
			"$lte": bson.A{
				bson.M{"$add": bson.A{"$usedStorageBytes", sizeBytes}},
				"$storageQuotaBytes",
			},
		},
	}
	update := bson.M{
		"$inc": bson.M{"usedStorageBytes": sizeBytes},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updated domain.User
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &updated, true, nil
}

// ReleaseStorageQuota vraća rezervisani prostor u slučaju neuspelog upisa fajla.
func (r *UserRepository) ReleaseStorageQuota(ctx context.Context, userID primitive.ObjectID, sizeBytes int64) error {
	filter := bson.M{"_id": userID}
	update := bson.M{
		"$inc": bson.M{"usedStorageBytes": -sizeBytes},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

// #endregion