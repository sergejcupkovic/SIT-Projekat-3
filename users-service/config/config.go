package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port              string
	MongoURI          string
	MongoDBName       string
	SMTPHost          string
	SMTPPort          string
	SMTPFrom          string
	DefaultQuotaBytes int64
}

func LoadConfig() *Config {
	port := os.Getenv("USERS_SERVICE_PORT")
	if port == "" {
		port = "8081"
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://mongodb:27017"
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "users_db"
	}

	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "mailpit"
	}

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "1025"
	}

	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "noreply@cloudstorage.local"
	}

	quotaStr := os.Getenv("DEFAULT_STORAGE_QUOTA_BYTES")
	quotaBytes, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil || quotaBytes <= 0 {
		quotaBytes = 104857600 // 100 MB default
	}

	return &Config{
		Port:              port,
		MongoURI:          mongoURI,
		MongoDBName:       dbName,
		SMTPHost:          smtpHost,
		SMTPPort:          smtpPort,
		SMTPFrom:          smtpFrom,
		DefaultQuotaBytes: quotaBytes,
	}
}
