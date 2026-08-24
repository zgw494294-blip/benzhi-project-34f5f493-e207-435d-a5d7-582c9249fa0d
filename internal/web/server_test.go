package web

import (
	"net/http"
	"net/http/httptest"
	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/storage"
	"strings"
	"testing"
)

func TestHomeAndJSONValidation(t *testing.T) {
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(application.New(repo, audit.New())).Handler()
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "临时活动用电送电许可工作台") {
		t.Fatalf("unexpected home: %d", home.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader("{}"))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", response.Code)
	}
	badStatus := httptest.NewRecorder()
	handler.ServeHTTP(badStatus, httptest.NewRequest(http.MethodGet, "/api/cases?status=UNKNOWN", nil))
	if badStatus.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", badStatus.Code)
	}
}
