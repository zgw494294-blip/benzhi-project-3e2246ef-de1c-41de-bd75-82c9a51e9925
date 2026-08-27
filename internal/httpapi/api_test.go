package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(application.NewService(repository, redaction.New()), logger)
}

func TestHealthAndMethodHandling(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("健康检查返回 %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("缺少安全缓存头")
	}
	request = httptest.NewRequest(http.MethodDelete, "/healthz", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("方法错误返回 %d", response.Code)
	}
}

func TestCommandStrictJSON(t *testing.T) {
	handler := testHandler(t)
	body := `{"command":"create_case","expectedRevision":0,"idempotencyKey":"k","actor":{"id":"a","role":"archivist"},"payload":{},"unknown":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cases/c/commands", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("未知字段返回 %d: %s", response.Code, response.Body.String())
	}
}
