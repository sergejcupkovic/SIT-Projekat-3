package service

import (
	"context"
	"log"
	"time"

	userspb "stashr/proto/users"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Quota struct {
	// Only meaningful for Reserve: false = the file does not fit, nothing was reserved.
	Allowed        bool
	QuotaBytes     int64
	UsedBytes      int64
	RemainingBytes int64
}

type QuotaClient struct {
	client  userspb.QuotaServiceClient
	timeout time.Duration
}

func NewQuotaClient(conn grpc.ClientConnInterface) *QuotaClient {
	return &QuotaClient{client: userspb.NewQuotaServiceClient(conn), timeout: 5 * time.Second}
}

func (c *QuotaClient) Get(ctx context.Context) (Quota, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()

	resp, err := c.client.GetQuota(ctx, &userspb.GetQuotaRequest{})
	if err != nil {
		return Quota{}, err
	}
	return fromProto(resp), nil
}

func (c *QuotaClient) Reserve(ctx context.Context, sizeBytes int64) (Quota, error) {
	ctx, cancel := c.call(ctx)
	defer cancel()

	resp, err := c.client.ReserveQuota(ctx, &userspb.ReserveQuotaRequest{SizeBytes: sizeBytes})
	if err != nil {
		return Quota{}, err
	}
	return fromProto(resp), nil
}

// Must also work when the request was cancelled, which is when rollbacks happen.
func (c *QuotaClient) Release(ctx context.Context, sizeBytes int64) {
	ctx, cancel := c.call(context.WithoutCancel(ctx))
	defer cancel()

	if _, err := c.client.ReleaseQuota(ctx, &userspb.ReleaseQuotaRequest{SizeBytes: sizeBytes}); err != nil {
		log.Printf("WARNING: could not release %d reserved quota bytes (quota stays reserved): %v", sizeBytes, err)
	}
}

func (c *QuotaClient) call(ctx context.Context) (context.Context, context.CancelFunc) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if auth := md.Get("authorization"); len(auth) > 0 {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", auth[0])
		}
	}
	return context.WithTimeout(ctx, c.timeout)
}

func fromProto(r *userspb.QuotaResponse) Quota {
	return Quota{
		Allowed:        r.GetAllowed(),
		QuotaBytes:     r.GetStorageQuotaBytes(),
		UsedBytes:      r.GetUsedStorageBytes(),
		RemainingBytes: r.GetRemainingBytes(),
	}
}
