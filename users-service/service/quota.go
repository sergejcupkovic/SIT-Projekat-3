package service

import (
	"errors"
	"fmt"
	"strings"
)

func QuotaInRange(bytes int64) bool {
	return bytes >= Cfg.MinQuotaBytes && bytes <= Cfg.MaxQuotaBytes
}

func QuotaRangeMessage() string {
	return fmt.Sprintf("Storage quota must be between %s and %s",
		FormatBytes(Cfg.MinQuotaBytes), FormatBytes(Cfg.MaxQuotaBytes))
}

// ValidateQuotaRequest checks the size a user asks for against their current quota.
func ValidateQuotaRequest(requested, current int64) error {
	if !QuotaInRange(requested) {
		return errors.New(QuotaRangeMessage())
	}
	if requested <= current {
		return errors.New("Requested quota must be greater than your current quota of " + FormatBytes(current))
	}
	return nil
}

// FormatBytes turns 104857600 into "100 MB".
func FormatBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value, i := float64(n), 0
	for value >= 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0") + " " + units[i]
}
