package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	userspb "stashr/proto/users"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRepositoryConfigLoads(t *testing.T) {
	limits, cfg, err := loadConfig(filepath.Join("..", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Upload.ChunkBytes != 64<<10 || cfg.Upload.Timeout != 10*time.Minute || cfg.RequestTimeout != 15*time.Second {
		t.Fatalf("unexpected values: %+v", cfg)
	}
	if limits.MaxUploadBytes != 200<<20 {
		t.Fatalf("max_upload_bytes = %d", limits.MaxUploadBytes)
	}
}

func TestConfigFileIsTheOnlySource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	write := func(content string) {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("rate_limit_burst: 7\nrate_limit_per_second: 2.5\nupload_timeout: 2m\ncors_allowed_origin: https://app.test\n")
	// Variables older .env files still carry must not override the file.
	t.Setenv("RATE_LIMIT_PER_SECOND", "99")
	t.Setenv("RATE_LIMIT_BURST", "99")
	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimitBurst != 7 || cfg.RateLimitPerSecond != 2.5 || cfg.Upload.Timeout != 2*time.Minute ||
		cfg.CORSAllowedOrigin != "https://app.test" {
		t.Fatalf("unexpected values: %+v", cfg)
	}
	// A key that is left out keeps its default.
	if cfg.Upload.ChunkBytes != 64<<10 {
		t.Fatalf("chunk size default lost: %d", cfg.Upload.ChunkBytes)
	}

	for _, bad := range []string{"rate_limit_burst: 0\n", "upload_chunk_bytes: 5000000\n", "gateway_read_timeout: 0s\n", "gateway_shutdown_timeout: 0s\n", "cors_allowed_origin: \"\"\n"} {
		write(bad)
		if _, _, err := loadConfig(path); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestLimitRequestBody(t *testing.T) {
	h := limitRequestBody(10, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	send := func(method, path string, chunked bool) int {
		req := httptest.NewRequest(method, path, strings.NewReader(strings.Repeat("a", 11)))
		if chunked {
			req.ContentLength = -1
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := send(http.MethodPost, "/api/users/login", false); code != http.StatusRequestEntityTooLarge {
		t.Errorf("declared length: got %d", code)
	}
	if code := send(http.MethodPost, "/api/users/login", true); code != http.StatusBadRequest {
		t.Errorf("unknown length: got %d", code)
	}
	for _, path := range []string{"/api/files/upload", "/api/files/abc/version"} {
		if code := send(http.MethodPost, path, false); code != http.StatusOK {
			t.Errorf("%s must be exempt, got %d", path, code)
		}
	}
	if code := send(http.MethodPatch, "/api/files/abc/version", false); code != http.StatusRequestEntityTooLarge {
		t.Errorf("only the POST upload routes are exempt, got %d", code)
	}
}

type unusedUsers struct {
	userspb.UnimplementedUsersServiceServer
}

type unusedQuotaRequests struct {
	userspb.UnimplementedQuotaRequestServiceServer
}

// A quota the JSON decoder cannot put into an int64 must still be answered with
// the range message, on both routes that accept a quota.
func TestUndecodableQuotaGetsTheRangeMessage(t *testing.T) {
	const message = "Storage quota must be between 100 MB and 2 TB"
	gw := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		}),
		runtime.WithErrorHandler(quotaErrorHandler(message)),
	)
	if err := userspb.RegisterUsersServiceHandlerServer(context.Background(), gw, unusedUsers{}); err != nil {
		t.Fatal(err)
	}
	if err := userspb.RegisterQuotaRequestServiceHandlerServer(context.Background(), gw, unusedQuotaRequests{}); err != nil {
		t.Fatal(err)
	}

	call := func(method, path, body string) (int, string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		var out struct{ Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out.Message
	}

	for _, value := range []string{"1e30", "1.048576e+36", "99999999999999999999999", "104857600.5", `"abc"`} {
		for _, c := range [][3]string{
			{http.MethodPatch, "/api/admin/users/abc/quota", `{"storageQuotaBytes":` + value + `}`},
			{http.MethodPost, "/api/users/quota-requests", `{"requestedQuotaBytes":` + value + `}`},
		} {
			code, got := call(c[0], c[1], c[2])
			if code != http.StatusBadRequest || got != message {
				t.Errorf("%s %s: got %d %q", c[1], c[2], code, got)
			}
		}
	}

	// Other decoding errors keep their own message.
	if _, got := call(http.MethodPost, "/api/users/login", `{"email":5}`); got == message {
		t.Errorf("unrelated error was rewritten: %q", got)
	}
}
