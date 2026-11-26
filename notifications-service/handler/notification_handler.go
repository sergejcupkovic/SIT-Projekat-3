package handler

import (
	"context"

	"notifications-service/repository"

	notificationspb "stashr/proto/notifications"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NotificationHandler struct {
	notificationspb.UnimplementedNotificationServiceServer
	repo *repository.CassandraNotificationRepository
}

func NewNotificationHandler(repo *repository.CassandraNotificationRepository) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

func (h *NotificationHandler) GetNotifications(ctx context.Context, _ *notificationspb.GetNotificationsRequest) (*notificationspb.GetNotificationsResponse, error) {
	items, err := h.repo.ListForAllUsers(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to load notifications")
	}

	resp := &notificationspb.GetNotificationsResponse{
		Notifications: make([]*notificationspb.NotificationItem, 0, len(items)),
	}
	for _, n := range items {
		resp.Notifications = append(resp.Notifications, &notificationspb.NotificationItem{
			Id:        n.ID,
			Title:     n.Title,
			Message:   n.Message,
			Type:      n.Type,
			IsRead:    n.IsRead,
			CreatedAt: timestamppb.New(n.CreatedAt),
		})
	}
	return resp, nil
}