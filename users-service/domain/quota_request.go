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

// QuotaRequest is a regular user's request for a larger storage quota
// (collection "quota_requests"). ResolvedAt and ResolvedBy (the administrator)
// stay empty while it is pending.
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
