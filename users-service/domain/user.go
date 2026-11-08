package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	RoleRegular Role = "RK"
	RoleAdmin   Role = "A"
)

type User struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FirstName                string             `bson:"firstName" json:"firstName"`
	LastName                 string             `bson:"lastName" json:"lastName"`
	Email                    string             `bson:"email" json:"email"`
	PasswordHash             string             `bson:"passwordHash" json:"-"`
	Role                     Role               `bson:"role" json:"role"`
	IsVerified               bool               `bson:"isVerified" json:"isVerified"`
	VerificationToken        string             `bson:"verificationToken,omitempty" json:"-"`
	VerificationTokenExpires time.Time          `bson:"verificationTokenExpires,omitempty" json:"-"`
	StorageQuotaBytes        int64              `bson:"storageQuotaBytes" json:"storageQuotaBytes"`
	UsedStorageBytes         int64              `bson:"usedStorageBytes" json:"usedStorageBytes"`
	PasswordLastChanged      time.Time          `bson:"passwordLastChanged" json:"passwordLastChanged"`
	CreatedAt                time.Time          `bson:"createdAt" json:"createdAt"`
}
