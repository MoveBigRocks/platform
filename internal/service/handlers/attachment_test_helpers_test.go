package servicehandlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
	"github.com/movebigrocks/platform/pkg/logger"
)

type capturedS3PutRequest struct {
	Method      string
	Path        string
	ContentType string
	Body        []byte
}

type testS3Server struct {
	server *httptest.Server
	mu     sync.Mutex
	puts   []capturedS3PutRequest
}

func newTestAttachmentService(t testing.TB) (*serviceapp.AttachmentService, *testS3Server) {
	t.Helper()

	s3Server := newTestS3Server(t)
	service, err := serviceapp.NewAttachmentService(serviceapp.AttachmentServiceConfig{
		S3Endpoint:  s3Server.URL(),
		S3Region:    "us-east-1",
		S3Bucket:    "mbr-test-attachments",
		S3AccessKey: "test-access-key",
		S3SecretKey: "test-secret-key",
		Logger:      logger.NewNop(),
	})
	if err != nil {
		t.Fatalf("create attachment service: %v", err)
	}
	return service, s3Server
}

func newTestS3Server(t testing.TB) *testS3Server {
	t.Helper()

	s3 := &testS3Server{}
	s3.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s3.mu.Lock()
		s3.puts = append(s3.puts, capturedS3PutRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		s3.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s3.server.Close)
	return s3
}

func (s *testS3Server) URL() string {
	return s.server.URL
}

func (s *testS3Server) PutRequests() []capturedS3PutRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]capturedS3PutRequest, len(s.puts))
	copy(result, s.puts)
	return result
}
