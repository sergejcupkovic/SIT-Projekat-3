package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type QuotaClient struct {
	baseURL    string
	httpClient *http.Client
}

type QuotaResponse struct {
	Allowed           bool   `json:"allowed"`
	Message           string `json:"message"`
	StorageQuotaBytes int64  `json:"storageQuotaBytes"`
	UsedStorageBytes  int64  `json:"usedStorageBytes"`
	RemainingBytes    int64  `json:"remainingBytes"`
}

func NewQuotaClient(baseURL string) *QuotaClient {
	return &QuotaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *QuotaClient) ReserveQuota(ctx context.Context, userID string, sizeBytes int64) (*QuotaResponse, int, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"userId":    userID,
		"sizeBytes": sizeBytes,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/users/quota/reserve", bytes.NewBuffer(payload))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("users service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var qResp QuotaResponse
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	return &qResp, resp.StatusCode, nil
}

func (c *QuotaClient) ReleaseQuota(ctx context.Context, userID string, sizeBytes int64) {
	payload, _ := json.Marshal(map[string]interface{}{
		"userId":    userID,
		"sizeBytes": sizeBytes,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/users/quota/release", bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func (c *QuotaClient) GetQuota(ctx context.Context, userID string) (*QuotaResponse, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/users/quota?userId="+userID, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("users service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var qResp QuotaResponse
	_ = json.NewDecoder(resp.Body).Decode(&qResp)
	return &qResp, resp.StatusCode, nil
}