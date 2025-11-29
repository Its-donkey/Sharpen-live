package adminhttp

import (
	"context"
	"encoding/json"
	"errors"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandlerAuthorizesRequests(t *testing.T) {
	handler := NewStatusHandler(StatusHandlerOptions{
		Authorizer: stubAuthorizer{err: errors.New("denied")},
		Service:    stubStatusService{},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/streamers/status", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestStatusHandlerValidatesMethod(t *testing.T) {
	handler := NewStatusHandler(StatusHandlerOptions{
		Authorizer: stubAuthorizer{},
		Service:    stubStatusService{},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/streamers/status", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
	if allow := rr.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow header %q, got %q", http.MethodPost, allow)
	}
}

func TestStatusHandlerReturnsResult(t *testing.T) {
	handler := NewStatusHandler(StatusHandlerOptions{
		Authorizer: stubAuthorizer{},
		Service: stubStatusService{
			result: adminservice.StatusCheckResult{Checked: 3, Online: 1},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/streamers/status", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp adminservice.StatusCheckResult
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Checked != 3 || resp.Online != 1 {
		t.Fatalf("unexpected payload: %+v", resp)
	}
}

type stubStatusService struct {
	result adminservice.StatusCheckResult
	err    error
}

func (s stubStatusService) CheckAll(context.Context) (adminservice.StatusCheckResult, error) {
	return s.result, s.err
}

type stubAuthorizer struct {
	err error
}

func (a stubAuthorizer) AuthorizeRequest(*http.Request) error {
	return a.err
}
