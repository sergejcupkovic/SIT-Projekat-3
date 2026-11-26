package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type QuotaRequestStatus string

const (
	QuotaRequestPending  QuotaRequestStatus = "pending"
	QuotaRequestApproved QuotaRequestStatus = "approved"
	QuotaRequestRejected QuotaRequestStatus = "rejected"
)

type QuotaRequest struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	UserID              primitive.ObjectID `bson:"userId"`
	QuotaAtRequestBytes int64              `bson:"quotaAtRequestBytes"`
	RequestedQuotaBytes int64              `bson:"requestedQuotaBytes"`
	Status              QuotaRequestStatus `bson:"status"`
	CreatedAt           time.Time          `bson:"createdAt"`
	ResolvedAt          time.Time          `bson:"resolvedAt,omitempty"`
	ResolvedBy          string             `bson:"resolvedBy,omitempty"`
}
