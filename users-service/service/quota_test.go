package service

import "testing"

func TestQuotaRange(t *testing.T) {
	min, max := Cfg.MinQuotaBytes, Cfg.MaxQuotaBytes
	if !QuotaInRange(min) || !QuotaInRange(max) {
		t.Error("the bounds themselves are allowed")
	}
	if QuotaInRange(min-1) || QuotaInRange(max+1) {
		t.Error("values outside the bounds are not allowed")
	}
	if got := QuotaRangeMessage(); got != "Storage quota must be between 100 MB and 2 TB" {
		t.Errorf("unexpected message: %q", got)
	}
}

func TestValidateQuotaRequest(t *testing.T) {
	const current = 200 << 20
	if err := ValidateQuotaRequest(1<<30, current); err != nil {
		t.Errorf("a larger quota is allowed: %v", err)
	}
	for name, requested := range map[string]int64{
		"same as current":      current,
		"smaller than current": current - 1,
		"above the maximum":    Cfg.MaxQuotaBytes + 1,
	} {
		if ValidateQuotaRequest(requested, current) == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
