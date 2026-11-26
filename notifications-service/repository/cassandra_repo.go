package repository

import (
	"context"
	"fmt"
	"time"

	"notifications-service/domain"

	"github.com/gocql/gocql"
)

const globalAudience = "ALL_RK"

type CassandraNotificationRepository struct {
	session *gocql.Session
}

func NewCassandraNotificationRepository(hosts []string, keyspace string) (*CassandraNotificationRepository, error) {
	sysCluster := gocql.NewCluster(hosts...)
	sysCluster.Consistency = gocql.Quorum
	sysCluster.Timeout = 10 * time.Second
	sysCluster.ConnectTimeout = 10 * time.Second

	sysSession, err := sysCluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect to cassandra: %w", err)
	}

	createKS := fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
	`, keyspace)
	if err := sysSession.Query(createKS).Exec(); err != nil {
		sysSession.Close()
		return nil, fmt.Errorf("create keyspace: %w", err)
	}
	sysSession.Close()

	ksCluster := gocql.NewCluster(hosts...)
	ksCluster.Keyspace = keyspace
	ksCluster.Consistency = gocql.One
	ksCluster.Timeout = 10 * time.Second
	ksCluster.ConnectTimeout = 10 * time.Second

	session, err := ksCluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect to keyspace %s: %w", keyspace, err)
	}

	createTable := `
		CREATE TABLE IF NOT EXISTS notifications_by_audience (
			audience text,
			created_at timestamp,
			id text,
			title text,
			message text,
			type text,
			is_read boolean,
			PRIMARY KEY ((audience), created_at, id)
		) WITH CLUSTERING ORDER BY (created_at DESC, id ASC)
	`
	if err := session.Query(createTable).Exec(); err != nil {
		session.Close()
		return nil, fmt.Errorf("create notifications table: %w", err)
	}

	repo := &CassandraNotificationRepository{session: session}
	if err := repo.seedIfEmpty(context.Background()); err != nil {
		session.Close()
		return nil, fmt.Errorf("seed notifications: %w", err)
	}

	return repo, nil
}

func (r *CassandraNotificationRepository) Close() {
	if r.session != nil {
		r.session.Close()
	}
}

func (r *CassandraNotificationRepository) seedIfEmpty(ctx context.Context) error {
	var existingID string
	iter := r.session.Query(
		`SELECT id FROM notifications_by_audience WHERE audience = ? LIMIT 1`,
		globalAudience,
	).WithContext(ctx).Iter()
	hasRow := iter.Scan(&existingID)
	if err := iter.Close(); err != nil {
		return err
	}
	if hasRow {
		return nil
	}

	now := time.Now().UTC()
	seeds := []domain.Notification{
		{
			ID:        "seed-1",
			Title:     "Welcome to Stashr Cloud Storage",
			Message:   "Your personal storage vault is active. Upload files, organize folders, and manage file versions anytime.",
			Type:      "success",
			IsRead:    false,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:        "seed-2",
			Title:     "Security Reminder: 2FA Enabled",
			Message:   "Two-factor authentication via one-time password protects every sign-in to your Stashr account.",
			Type:      "info",
			IsRead:    false,
			CreatedAt: now.Add(-45 * time.Minute),
		},
		{
			ID:        "seed-3",
			Title:     "Supported File Types & Limits",
			Message:   "You can store .txt, .md, .csv, .json, .pdf, .png, .jpg, and .gif files. Need more space? Use \"Request More Storage\".",
			Type:      "warning",
			IsRead:    false,
			CreatedAt: now.Add(-10 * time.Minute),
		},
	}

	for _, n := range seeds {
		err := r.session.Query(
			`INSERT INTO notifications_by_audience (audience, created_at, id, title, message, type, is_read)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			globalAudience, n.CreatedAt, n.ID, n.Title, n.Message, n.Type, n.IsRead,
		).WithContext(ctx).Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *CassandraNotificationRepository) ListForAllUsers(ctx context.Context) ([]domain.Notification, error) {
	iter := r.session.Query(
		`SELECT id, title, message, type, is_read, created_at
		 FROM notifications_by_audience
		 WHERE audience = ?`,
		globalAudience,
	).WithContext(ctx).Iter()

	var result []domain.Notification
	var n domain.Notification
	for iter.Scan(&n.ID, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt) {
		result = append(result, n)
		n = domain.Notification{}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return result, nil
}