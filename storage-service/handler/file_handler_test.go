package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"storage-service/domain"
	"storage-service/middleware"
	"storage-service/service"

	storagepb "stashr/proto/storage"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

var jwtSecret = []byte("test-secret-test-secret-test-secret-test")

// ---------- fakes ----------

// env is the whole fake world of one test. `events` records the order in
// which the handler touched the collaborators.
type env struct {
	mu sync.Mutex

	events []string

	// users-service
	quota, used int64
	quotaErr    error

	// object store
	objects map[string][]byte
	putErr  error

	// MongoDB
	files     []*domain.FileItem
	folders   map[primitive.ObjectID]*domain.Folder
	createErr error
}

func (e *env) log(ev string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
}

func (e *env) eventList() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.events, ",")
}

func (e *env) usedBytes() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.used
}

func (e *env) quotaView() service.Quota {
	rem := e.quota - e.used
	if rem < 0 {
		rem = 0
	}
	return service.Quota{QuotaBytes: e.quota, UsedBytes: e.used, RemainingBytes: rem}
}

// service.QuotaClient stand-in
func (e *env) Get(context.Context) (service.Quota, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.quotaErr != nil {
		return service.Quota{}, e.quotaErr
	}
	return e.quotaView(), nil
}

func (e *env) Reserve(_ context.Context, size int64) (service.Quota, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, "reserve")
	if e.quotaErr != nil {
		return service.Quota{}, e.quotaErr
	}
	if e.used+size > e.quota {
		return e.quotaView(), nil // Allowed: false
	}
	e.used += size
	q := e.quotaView()
	q.Allowed = true
	return q, nil
}

func (e *env) Release(_ context.Context, size int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, "release")
	e.used -= size
	if e.used < 0 {
		e.used = 0
	}
}

// objectStore stand-in: like minio-go it reads exactly `size` bytes and fails
// if the reader ends sooner.
func (e *env) Put(_ context.Context, key string, r io.Reader, size int64, _ string) error {
	e.log("put")
	if e.putErr != nil {
		return e.putErr
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	e.mu.Lock()
	e.objects[key] = buf
	e.mu.Unlock()
	return nil
}

func (e *env) Delete(_ context.Context, key string) error {
	e.log("delete")
	e.mu.Lock()
	delete(e.objects, key)
	e.mu.Unlock()
	return nil
}

// fileRepository stand-in
func (e *env) Create(_ context.Context, f *domain.FileItem) error {
	e.log("create")
	if e.createErr != nil {
		return e.createErr
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.files = append(e.files, f)
	return nil
}

func (e *env) ExistsByNameInFolder(_ context.Context, owner string, folder *primitive.ObjectID, name string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range e.files {
		if f.OwnerID == owner && f.Name == name && sameFolder(f.FolderID, folder) {
			return true, nil
		}
	}
	return false, nil
}

func (e *env) FindByFolder(_ context.Context, owner string, folder *primitive.ObjectID) ([]domain.FileItem, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []domain.FileItem{}
	for _, f := range e.files {
		if f.OwnerID == owner && sameFolder(f.FolderID, folder) {
			out = append(out, *f)
		}
	}
	return out, nil
}

// folderLookup stand-in
func (e *env) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Folder, error) {
	f, ok := e.folders[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return f, nil
}

func sameFolder(a, b *primitive.ObjectID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ---------- harness ----------

const (
	alice = "aaaaaaaaaaaaaaaaaaaaaaaa"
	bob   = "bbbbbbbbbbbbbbbbbbbbbbbb"
)

type testServer struct {
	storagepb.UnimplementedStorageServiceServer
	fh *FileHandler
}

func (s testServer) UploadFile(st storagepb.StorageService_UploadFileServer) error {
	return s.fh.UploadFile(st)
}

func (s testServer) GetQuota(ctx context.Context, r *storagepb.GetQuotaRequest) (*storagepb.StorageQuota, error) {
	return s.fh.GetQuota(ctx, r)
}

func newEnv(quota, used int64) *env {
	return &env{
		quota:   quota,
		used:    used,
		objects: map[string][]byte{},
		folders: map[primitive.ObjectID]*domain.Folder{},
	}
}

// start runs a real gRPC server (with the real JWT stream interceptor) over an
// in-memory connection and returns a client for it.
func start(t *testing.T, e *env, maxFileBytes int64) storagepb.StorageServiceClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(middleware.AuthUnaryInterceptor(jwtSecret)),
		grpc.ChainStreamInterceptor(middleware.AuthStreamInterceptor(jwtSecret)),
	)
	storagepb.RegisterStorageServiceServer(srv, testServer{fh: NewFileHandler(e, e, e, e, maxFileBytes)})
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})
	return storagepb.NewStorageServiceClient(conn)
}

func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "email": userID + "@example.com", "role": "RK",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwtSecret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func authCtx(t *testing.T, userID string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tokenFor(t, userID))
}

// upload sends metadata followed by the given chunks and returns the reply.
func upload(ctx context.Context, c storagepb.StorageServiceClient, meta *storagepb.UploadFileMetadata, chunks ...[]byte) (*storagepb.UploadFileResponse, error) {
	stream, err := c.UploadFile(ctx)
	if err != nil {
		return nil, err
	}
	send := func(m *storagepb.UploadFileRequest) bool {
		// io.EOF means the server already ended the call; the real status comes from CloseAndRecv.
		return !errors.Is(stream.Send(m), io.EOF)
	}
	if send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Metadata{Metadata: meta}}) {
		for _, ch := range chunks {
			if !send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Chunk{Chunk: ch}}) {
				break
			}
		}
	}
	return stream.CloseAndRecv()
}

func simpleUpload(ctx context.Context, c storagepb.StorageServiceClient, name string, content []byte) (*storagepb.UploadFileResponse, error) {
	return upload(ctx, c, &storagepb.UploadFileMetadata{FileName: name, SizeBytes: int64(len(content))}, content)
}

func wantCode(t *testing.T, err error, want codes.Code) *status.Status {
	t.Helper()
	st, _ := status.FromError(err)
	if st.Code() != want {
		t.Fatalf("expected %v, got %v (%v)", want, st.Code(), err)
	}
	return st
}

// ---------- US-2.2 happy path ----------

func TestUpload_StoresContentAndMetadata(t *testing.T) {
	e := newEnv(1000, 100)
	folderID := primitive.NewObjectID()
	e.folders[folderID] = &domain.Folder{ID: folderID, OwnerID: alice, Name: "Docs"}
	c := start(t, e, 0)

	content := []byte("hello from the upload test")
	hex := folderID.Hex()
	resp, err := upload(authCtx(t, alice), c,
		&storagepb.UploadFileMetadata{FileName: "notes.txt", FolderId: &hex, SizeBytes: int64(len(content))},
		content[:10], content[10:])
	if err != nil {
		t.Fatal(err)
	}

	// 2.2.2 before 2.2.3: the quota is checked before anything is stored or saved.
	if got := e.eventList(); got != "reserve,put,create" {
		t.Fatalf("expected quota check -> object store -> metadata, got %s", got)
	}

	if len(e.files) != 1 {
		t.Fatalf("expected 1 metadata document, got %d", len(e.files))
	}
	f := e.files[0]
	if f.Name != "notes.txt" || f.OwnerID != alice || f.SizeBytes != int64(len(content)) ||
		f.MimeType != "text/plain" || f.FolderID == nil || *f.FolderID != folderID || f.CreatedAt.IsZero() {
		t.Fatalf("metadata is incomplete or wrong: %+v", f)
	}
	if !bytes.Equal(e.objects[f.ObjectKey], content) {
		t.Fatal("object store content differs from the uploaded content")
	}
	sum := sha256.Sum256(content)
	if f.SHA256 != hexOf(sum[:]) {
		t.Fatalf("stored SHA-256 %s does not match the content", f.SHA256)
	}
	if !strings.HasPrefix(f.ObjectKey, alice+"/") {
		t.Fatalf("object key should be namespaced by owner, got %s", f.ObjectKey)
	}

	if resp.GetFile().GetId() != f.ID.Hex() || resp.GetFile().GetName() != "notes.txt" {
		t.Fatalf("unexpected file in reply: %v", resp.GetFile())
	}
	if resp.GetQuota().GetUsedStorageBytes() != 100+int64(len(content)) {
		t.Fatalf("reply must carry the updated usage, got %v", resp.GetQuota())
	}
	if e.usedBytes() != 100+int64(len(content)) {
		t.Fatalf("quota usage must stay reserved, got %d", e.usedBytes())
	}
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestUpload_RootFolderWhenNoneSelected(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	if _, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if e.files[0].FolderID != nil {
		t.Fatal("a file without a target folder belongs to the root (folderId nil)")
	}
}

func TestUpload_LargeMultiChunkFile(t *testing.T) {
	e := newEnv(10<<20, 0)
	c := start(t, e, 0)

	content := bytes.Repeat([]byte("0123456789abcdef"), 1<<16) // 1 MiB
	var chunks [][]byte
	for off := 0; off < len(content); off += 64 << 10 {
		chunks = append(chunks, content[off:off+64<<10])
	}
	if _, err := upload(authCtx(t, alice), c,
		&storagepb.UploadFileMetadata{FileName: "big.txt", SizeBytes: int64(len(content))}, chunks...); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(e.objects[e.files[0].ObjectKey], content) {
		t.Fatal("1 MiB content was corrupted on the way")
	}
}

func TestUpload_SmallPNGKeepsDetectedType(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01")
	if _, err := simpleUpload(authCtx(t, alice), c, "pic.png", png); err != nil {
		t.Fatal(err)
	}
	if e.files[0].MimeType != "image/png" {
		t.Fatalf("type must be detected from the content, got %q", e.files[0].MimeType)
	}
}

// ---------- 2.2.2 / 2.2.4: quota ----------

func TestUpload_QuotaExceededAbortsWithClearMessageAndStoresNothing(t *testing.T) {
	e := newEnv(100, 90) // 10 bytes free
	c := start(t, e, 0)

	_, err := simpleUpload(authCtx(t, alice), c, "big.txt", bytes.Repeat([]byte("x"), 11))
	st := wantCode(t, err, codes.ResourceExhausted)

	for _, want := range []string{"Storage quota exceeded", "11 B", "10 B", "nothing was saved"} {
		if !strings.Contains(st.Message(), want) {
			t.Errorf("message %q should mention %q", st.Message(), want)
		}
	}
	if got := e.eventList(); got != "reserve" {
		t.Fatalf("only the quota check may happen, got %s", got)
	}
	if len(e.objects) != 0 || len(e.files) != 0 {
		t.Fatal("nothing may be stored when the quota is exceeded")
	}
	if e.usedBytes() != 90 {
		t.Fatalf("usage must be unchanged, got %d", e.usedBytes())
	}
}

func TestUpload_FileExactlyFillingTheQuotaIsAllowed(t *testing.T) {
	e := newEnv(100, 90)
	c := start(t, e, 0)

	if _, err := simpleUpload(authCtx(t, alice), c, "fits.txt", bytes.Repeat([]byte("x"), 10)); err != nil {
		t.Fatalf("10 bytes fit exactly into 10 free bytes: %v", err)
	}
	if e.usedBytes() != 100 {
		t.Fatalf("expected usage 100, got %d", e.usedBytes())
	}
}

func TestUpload_UsersServiceDownRejectsUploadInsteadOfSkippingTheCheck(t *testing.T) {
	e := newEnv(1000, 0)
	e.quotaErr = errors.New("connection refused")
	c := start(t, e, 0)

	_, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("x"))
	wantCode(t, err, codes.Unavailable)
	if len(e.objects) != 0 || len(e.files) != 0 {
		t.Fatal("an unverified upload must not be stored")
	}
}

// ---------- rollback ----------

func TestUpload_ObjectStoreFailureRollsBackQuota(t *testing.T) {
	e := newEnv(1000, 100)
	e.putErr = errors.New("s3 down")
	c := start(t, e, 0)

	_, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("hello"))
	wantCode(t, err, codes.Internal)

	if got := e.eventList(); got != "reserve,put,delete,release" {
		t.Fatalf("expected reserve, put, then rollback, got %s", got)
	}
	if e.usedBytes() != 100 {
		t.Fatalf("reserved space must be returned, usage is %d", e.usedBytes())
	}
	if len(e.files) != 0 {
		t.Fatal("no metadata may be saved")
	}
}

func TestUpload_MetadataFailureRemovesObjectAndReleasesQuota(t *testing.T) {
	e := newEnv(1000, 100)
	e.createErr = errors.New("mongo down")
	c := start(t, e, 0)

	_, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("hello"))
	wantCode(t, err, codes.Internal)

	if got := e.eventList(); got != "reserve,put,create,delete,release" {
		t.Fatalf("unexpected sequence %s", got)
	}
	if len(e.objects) != 0 {
		t.Fatal("the stored object must be removed again")
	}
	if e.usedBytes() != 100 {
		t.Fatalf("usage must be back at 100, got %d", e.usedBytes())
	}
}

func TestUpload_DuplicateKeyRaceIsReportedAsConflictAndRolledBack(t *testing.T) {
	e := newEnv(1000, 0)
	e.createErr = mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000}}}
	c := start(t, e, 0)

	_, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("hello"))
	wantCode(t, err, codes.AlreadyExists)
	if e.usedBytes() != 0 || len(e.objects) != 0 {
		t.Fatal("rollback expected")
	}
}

func TestUpload_ClientDisconnectMidUploadRollsBack(t *testing.T) {
	e := newEnv(10000, 100)
	c := start(t, e, 0)

	ctx, cancel := context.WithCancel(authCtx(t, alice))
	stream, err := c.UploadFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Declare 5000 bytes, deliver 600 (enough for the server to sniff the type,
	// reserve the quota and start storing), then vanish.
	_ = stream.Send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Metadata{
		Metadata: &storagepb.UploadFileMetadata{FileName: "a.txt", SizeBytes: 5000}}})
	_ = stream.Send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Chunk{Chunk: bytes.Repeat([]byte("x"), 600)}})

	waitFor(t, "the upload to reach the object store", func() bool { return strings.HasSuffix(e.eventList(), "put") })
	if e.usedBytes() != 5100 {
		t.Fatalf("precondition: 5000 bytes must be reserved by now, usage is %d", e.usedBytes())
	}
	cancel()

	waitFor(t, "the rollback", func() bool { return strings.HasSuffix(e.eventList(), "release") })
	if got := e.eventList(); got != "reserve,put,delete,release" {
		t.Fatalf("expected reserve, put, then rollback, got %s", got)
	}
	if e.usedBytes() != 100 {
		t.Fatalf("a cancelled upload must give its reservation back, usage is %d", e.usedBytes())
	}
	if len(e.files) != 0 {
		t.Fatal("no metadata may be saved for a cancelled upload")
	}
}

// ---------- declared size is enforced ----------

func TestUpload_FewerBytesThanDeclaredIsRejected(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	_, err := upload(authCtx(t, alice), c,
		&storagepb.UploadFileMetadata{FileName: "a.txt", SizeBytes: 500}, []byte("only a few bytes"))
	wantCode(t, err, codes.InvalidArgument)
	if e.usedBytes() != 0 || len(e.files) != 0 || len(e.objects) != 0 {
		t.Fatal("rollback expected")
	}
}

func TestUpload_MoreBytesThanDeclaredIsRejected(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	// reserves 5 bytes of quota but tries to push 600
	_, err := upload(authCtx(t, alice), c,
		&storagepb.UploadFileMetadata{FileName: "a.txt", SizeBytes: 5}, []byte("hello"), bytes.Repeat([]byte("x"), 600))
	wantCode(t, err, codes.InvalidArgument)
	if e.usedBytes() != 0 || len(e.files) != 0 || len(e.objects) != 0 {
		t.Fatalf("rollback expected (used=%d files=%d objects=%d)", e.usedBytes(), len(e.files), len(e.objects))
	}
}

func TestUpload_SecondMetadataMessageIsRejected(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	stream, _ := c.UploadFile(authCtx(t, alice))
	m := &storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Metadata{
		Metadata: &storagepb.UploadFileMetadata{FileName: "a.txt", SizeBytes: 100}}}
	_ = stream.Send(m)
	_ = stream.Send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Chunk{Chunk: []byte("hello")}})
	_ = stream.Send(m)
	_, err := stream.CloseAndRecv()
	wantCode(t, err, codes.InvalidArgument)
	if e.usedBytes() != 0 {
		t.Fatal("rollback expected")
	}
}

func TestUpload_FirstMessageMustBeMetadata(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	stream, _ := c.UploadFile(authCtx(t, alice))
	_ = stream.Send(&storagepb.UploadFileRequest{Payload: &storagepb.UploadFileRequest_Chunk{Chunk: []byte("hello")}})
	_, err := stream.CloseAndRecv()
	wantCode(t, err, codes.InvalidArgument)
	if e.eventList() != "" {
		t.Fatalf("nothing may happen, got %s", e.eventList())
	}
}

// ---------- validation happens before quota/storage ----------

func TestUpload_RejectedInputNeverTouchesQuotaOrStore(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	exe := []byte("MZ\x90\x00\x03\x00\x00\x00")

	cases := []struct {
		name    string
		file    string
		content []byte
		size    int64
		want    codes.Code
		msg     string
	}{
		{"empty file", "a.txt", nil, 0, codes.InvalidArgument, "empty"},
		{"negative size", "a.txt", []byte("x"), -5, codes.InvalidArgument, "empty"},
		{"executable extension", "setup.exe", exe, int64(len(exe)), codes.InvalidArgument, "not allowed"},
		{"executable renamed to png", "photo.png", exe, int64(len(exe)), codes.InvalidArgument, "does not match"},
		{"path traversal in name", "../../etc/passwd.txt", []byte("x"), 1, codes.InvalidArgument, "Invalid file name"},
		{"script in name", "<img src=x>.png", png, int64(len(png)), codes.InvalidArgument, "Invalid file name"},
		{"too large", "a.txt", bytes.Repeat([]byte("x"), 51), 51, codes.InvalidArgument, "too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(1<<30, 0)
			c := start(t, e, 50)

			_, err := upload(authCtx(t, alice), c, &storagepb.UploadFileMetadata{FileName: tc.file, SizeBytes: tc.size}, tc.content)
			st := wantCode(t, err, tc.want)
			if !strings.Contains(st.Message(), tc.msg) {
				t.Errorf("message %q should contain %q", st.Message(), tc.msg)
			}
			if e.eventList() != "" || e.usedBytes() != 0 {
				t.Fatalf("no side effects expected, got events %q used %d", e.eventList(), e.usedBytes())
			}
		})
	}
}

func TestUpload_DuplicateNameInSameFolderIsRejectedBeforeQuota(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	if _, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("one")); err != nil {
		t.Fatal(err)
	}
	e.events = nil
	_, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("two"))
	wantCode(t, err, codes.AlreadyExists)
	if e.eventList() != "" {
		t.Fatalf("a duplicate must be rejected before any side effect, got %s", e.eventList())
	}

	// another user may use the same name
	if _, err := simpleUpload(authCtx(t, bob), c, "a.txt", []byte("bobs")); err != nil {
		t.Fatalf("same name for another user must be fine: %v", err)
	}
}

// ---------- target folder ----------

func TestUpload_TargetFolderChecks(t *testing.T) {
	e := newEnv(1000, 0)
	bobsFolder := primitive.NewObjectID()
	e.folders[bobsFolder] = &domain.Folder{ID: bobsFolder, OwnerID: bob, Name: "Private"}
	c := start(t, e, 0)

	cases := []struct {
		name   string
		folder string
		want   codes.Code
	}{
		{"someone else's folder", bobsFolder.Hex(), codes.PermissionDenied},
		{"unknown folder", primitive.NewObjectID().Hex(), codes.NotFound},
		{"malformed id", "not-an-id", codes.InvalidArgument},
		{"nosql operator as id", `{"$ne":""}`, codes.InvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			folder := tc.folder
			_, err := upload(authCtx(t, alice), c,
				&storagepb.UploadFileMetadata{FileName: "a.txt", FolderId: &folder, SizeBytes: 1}, []byte("x"))
			wantCode(t, err, tc.want)
		})
	}
	if e.eventList() != "" {
		t.Fatalf("a rejected target folder must not reserve quota or store anything, got %s", e.eventList())
	}
}

// ---------- authentication ----------

func TestUpload_RequiresAValidSessionToken(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	_, err := simpleUpload(context.Background(), c, "a.txt", []byte("x"))
	wantCode(t, err, codes.Unauthenticated)

	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": alice, "role": "RK", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("some-other-secret-some-other-secret"))
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+forged)
	_, err = simpleUpload(ctx, c, "a.txt", []byte("x"))
	wantCode(t, err, codes.Unauthenticated)

	if e.eventList() != "" {
		t.Fatalf("unauthenticated uploads must not reach the handler, got %s", e.eventList())
	}
}

func TestUpload_OwnerComesFromTheTokenNotTheRequest(t *testing.T) {
	e := newEnv(1000, 0)
	c := start(t, e, 0)

	if _, err := simpleUpload(authCtx(t, alice), c, "a.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if e.files[0].OwnerID != alice {
		t.Fatalf("owner must be the authenticated user, got %s", e.files[0].OwnerID)
	}
}

// ---------- GetQuota ----------

func TestGetQuota(t *testing.T) {
	e := newEnv(1000, 250)
	c := start(t, e, 0)

	q, err := c.GetQuota(authCtx(t, alice), &storagepb.GetQuotaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if q.GetStorageQuotaBytes() != 1000 || q.GetUsedStorageBytes() != 250 || q.GetRemainingBytes() != 750 {
		t.Fatalf("unexpected quota %v", q)
	}

	if _, err := c.GetQuota(context.Background(), &storagepb.GetQuotaRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetQuota requires a token, got %v", err)
	}

	e.quotaErr = errors.New("boom")
	_, err = c.GetQuota(authCtx(t, alice), &storagepb.GetQuotaRequest{})
	wantCode(t, err, codes.Unavailable)
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 11: "11 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 104857600: "100.0 MB", 5 << 30: "5.0 GB"} {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
