package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	storagepb "stashr/proto/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)


// received is what the fake storage-service saw of one upload.
type received struct {
	meta    *storagepb.UploadFileMetadata
	content []byte
	auth    string
	chunks  int
}

// fakeStorage answers by file name so each test can pick the outcome.
type fakeStorage struct {
	storagepb.UnimplementedStorageServiceServer

	mu   sync.Mutex
	last *received
}

func (f *fakeStorage) UploadFile(stream storagepb.StorageService_UploadFileServer) error {
	r := &received{}
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if v := md.Get("authorization"); len(v) > 0 {
			r.auth = v[0]
		}
	}

	first, err := stream.Recv()
	if err != nil {
		return err
	}
	r.meta = first.GetMetadata()

	f.mu.Lock()
	f.last = r
	f.mu.Unlock()

	switch r.meta.GetFileName() {
	case "quota.txt":
		return status.Error(codes.ResourceExhausted, "Storage quota exceeded: this file is 5 B but only 1 B is free")
	case "dup.txt":
		return status.Error(codes.AlreadyExists, "A file with this name already exists in the current folder")
	case "bad.exe":
		return status.Error(codes.InvalidArgument, "This file type is not allowed")
	case "down.txt":
		return status.Error(codes.Unavailable, "Could not verify your storage quota")
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		r.content = append(r.content, msg.GetChunk()...)
		r.chunks++
	}

	return stream.SendAndClose(&storagepb.UploadFileResponse{
		Message: "File uploaded successfully",
		File: &storagepb.FileItem{
			Id: "f1", Name: r.meta.GetFileName(), SizeBytes: int64(len(r.content)),
			MimeType: "text/plain", CreatedAt: timestamppb.New(time.Unix(1700000000, 0)),
		},
		Quota: &storagepb.StorageQuota{StorageQuotaBytes: 1000, UsedStorageBytes: 300, RemainingBytes: 700},
	})
}

func (f *fakeStorage) GetQuota(context.Context, *storagepb.GetQuotaRequest) (*storagepb.StorageQuota, error) {
	return &storagepb.StorageQuota{StorageQuotaBytes: 104857600, UsedStorageBytes: 2048, RemainingBytes: 104855552}, nil
}

func (f *fakeStorage) lastUpload() *received {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last
}

// newGateway wires the real grpc-gateway mux and the real upload route to a
// fake storage-service over an in-memory gRPC connection.
func newGateway(t *testing.T, maxBytes int64) (*httptest.Server, *fakeStorage) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	fake := &fakeStorage{}
	srv := grpc.NewServer()
	storagepb.RegisterStorageServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	gw := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions:   protojson.MarshalOptions{EmitUnpopulated: true},
		UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
	}))
	if err := storagepb.RegisterStorageServiceHandler(context.Background(), gw, conn); err != nil {
		t.Fatal(err)
	}
	up := newUploadHandler(gw, storagepb.NewStorageServiceClient(conn), testSecret, maxBytes)
	if err := gw.HandlePath(http.MethodPost, "/api/files/upload", up.handle); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(gw)
	t.Cleanup(func() {
		ts.Close()
		conn.Close()
		srv.Stop()
	})
	return ts, fake
}

func bearer(t *testing.T, key []byte, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + s
}

func validAuth(t *testing.T) string {
	return bearer(t, testSecret, jwt.MapClaims{"sub": "u1", "role": "RK"})
}

type field struct{ name, value string }

// post sends a multipart upload. fileName == "" omits the file part.
func post(t *testing.T, ts *httptest.Server, auth, fileName string, content []byte, fields ...field) (*http.Response, map[string]any) {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if fileName != "" {
		fw, _ := mw.CreateFormFile("file", fileName)
		_, _ = fw.Write(content)
	}
	for _, f := range fields { // fields AFTER the file part: order must not matter
		_ = mw.WriteField(f.name, f.value)
	}
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/files/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return do(t, req)
}

func do(t *testing.T, req *http.Request) (*http.Response, map[string]any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("response is not JSON (%d): %s", resp.StatusCode, raw)
	}
	return resp, out
}

func TestUpload_SuccessStreamsMetadataAndContentWith201(t *testing.T) {
	ts, fake := newGateway(t, 10<<20)

	// bigger than one 64 KiB chunk so the streaming is exercised
	content := bytes.Repeat([]byte("0123456789"), 20000) // 200,000 bytes
	resp, body := post(t, ts, validAuth(t), "report.txt", content, field{"folderId", "  64b0c0ffee0000000000abcd  "})

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	got := fake.lastUpload()
	if got.meta.GetFileName() != "report.txt" || got.meta.GetSizeBytes() != int64(len(content)) {
		t.Fatalf("metadata sent to storage is wrong: %v", got.meta)
	}
	if got.meta.GetFolderId() != "64b0c0ffee0000000000abcd" {
		t.Fatalf("folderId must be forwarded (trimmed), got %q", got.meta.GetFolderId())
	}
	if !bytes.Equal(got.content, content) || got.chunks < 3 {
		t.Fatalf("content corrupted or not chunked: %d bytes in %d chunks", len(got.content), got.chunks)
	}

	// same JSON conventions as the generated routes
	if body["message"] != "File uploaded successfully" {
		t.Fatalf("unexpected body: %v", body)
	}
	file := body["file"].(map[string]any)
	if file["name"] != "report.txt" || file["mimeType"] != "text/plain" {
		t.Fatalf("unexpected file: %v", file)
	}
	if file["sizeBytes"] != "200000" { // protojson: int64 as string
		t.Fatalf("sizeBytes should be the string \"200000\", got %#v", file["sizeBytes"])
	}
	if q := body["quota"].(map[string]any); q["usedStorageBytes"] != "300" || q["remainingBytes"] != "700" {
		t.Fatalf("unexpected quota: %v", q)
	}
}

func TestUpload_NoFolderMeansRoot(t *testing.T) {
	ts, fake := newGateway(t, 10<<20)

	if resp, body := post(t, ts, validAuth(t), "a.txt", []byte("x")); resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d: %v", resp.StatusCode, body)
	}
	if fake.lastUpload().meta.FolderId != nil {
		t.Fatal("without folderId the field must stay unset (root)")
	}
}

func TestUpload_TokenIsForwardedToStorageService(t *testing.T) {
	ts, fake := newGateway(t, 10<<20)

	auth := validAuth(t)
	post(t, ts, auth, "a.txt", []byte("x"))
	if fake.lastUpload().auth != auth {
		t.Fatalf("storage-service needs the caller's token, got %q", fake.lastUpload().auth)
	}
}

func TestUpload_QuotaExceededIs413WithTheMessage(t *testing.T) {
	ts, _ := newGateway(t, 10<<20)

	resp, body := post(t, ts, validAuth(t), "quota.txt", []byte("hello"))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("quota violation must be 413 (not 429, which means rate limiting), got %d", resp.StatusCode)
	}
	if !strings.Contains(body["message"].(string), "Storage quota exceeded") {
		t.Fatalf("the clear message must reach the client: %v", body)
	}
	if body["code"] != float64(codes.ResourceExhausted) || body["details"] == nil {
		t.Fatalf("error body must keep the grpc-gateway shape: %v", body)
	}
}

func TestUpload_StorageErrorsMapToHTTPStatuses(t *testing.T) {
	ts, _ := newGateway(t, 10<<20)

	for file, want := range map[string]int{
		"dup.txt":  http.StatusConflict,
		"bad.exe":  http.StatusBadRequest,
		"down.txt": http.StatusServiceUnavailable,
	} {
		resp, body := post(t, ts, validAuth(t), file, []byte("hello"))
		if resp.StatusCode != want || body["message"] == "" {
			t.Errorf("%s: expected %d with a message, got %d %v", file, want, resp.StatusCode, body)
		}
	}
}

func TestUpload_RejectsMissingOrForgedTokenBeforeReadingTheBody(t *testing.T) {
	ts, fake := newGateway(t, 10<<20)

	forged := bearer(t, []byte("some-other-secret-some-other-secret"), jwt.MapClaims{"sub": "u1"})
	expired := bearer(t, testSecret, jwt.MapClaims{"sub": "u1", "exp": time.Now().Add(-time.Minute).Unix()})

	for name, auth := range map[string]string{"none": "", "garbage": "Bearer nope", "forged": forged, "expired": expired, "not bearer": "Basic abc"} {
		resp, _ := post(t, ts, auth, "a.txt", []byte("x"))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, resp.StatusCode)
		}
	}
	if fake.lastUpload() != nil {
		t.Fatal("an unauthenticated request must never reach storage-service")
	}
}

func TestUpload_BadRequests(t *testing.T) {
	ts, fake := newGateway(t, 10<<20)

	t.Run("no file part", func(t *testing.T) {
		resp, body := post(t, ts, validAuth(t), "", nil, field{"folderId", "x"})
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body["message"].(string), "select a file") {
			t.Fatalf("got %d %v", resp.StatusCode, body)
		}
	})

	t.Run("not multipart", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/files/upload", strings.NewReader(`{"file":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", validAuth(t))
		resp, body := do(t, req)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("got %d %v", resp.StatusCode, body)
		}
	})

	if fake.lastUpload() != nil {
		t.Fatal("bad requests must not reach storage-service")
	}
}

func TestUpload_FilesOverTheLimitAreRejectedWith413(t *testing.T) {
	ts, fake := newGateway(t, 1000) // tiny limit

	resp, body := post(t, ts, validAuth(t), "big.txt", bytes.Repeat([]byte("x"), 4<<20))
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d %v", resp.StatusCode, body)
	}
	if !strings.Contains(body["message"].(string), "too large") {
		t.Fatalf("unexpected message: %v", body)
	}
	if fake.lastUpload() != nil {
		t.Fatal("an oversized file must not reach storage-service")
	}
}

func TestUpload_OnlyPostIsRouted(t *testing.T) {
	ts, _ := newGateway(t, 10<<20)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/files/upload", nil)
	req.Header.Set("Authorization", validAuth(t))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		t.Fatalf("GET must not upload, got %d", resp.StatusCode)
	}
}

// The frontend relies on this exact JSON: protojson sends int64 as strings.
func TestGetQuotaRouteJSONShape(t *testing.T) {
	ts, _ := newGateway(t, 10<<20)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/files/quota", nil)
	req.Header.Set("Authorization", validAuth(t))
	resp, body := do(t, req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d %v", resp.StatusCode, body)
	}
	want := map[string]any{"storageQuotaBytes": "104857600", "usedStorageBytes": "2048", "remainingBytes": "104855552"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %#v, want %#v", k, body[k], v)
		}
	}
}

func TestHumanSize(t *testing.T) {
	for in, want := range map[int64]string{500: "500 B", 1536: "1.5 KB", 209715200: "200.0 MB"} {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", in, got, want)
		}
	}
}
