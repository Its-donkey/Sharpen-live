// file name — /internal/ui/server/admin_handlers_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAdminHome_OK(t *testing.T) {
	// TODO: implement newTestDependencies to construct server deps with fakes
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	// TODO: call the real handler once dependencies are available
	_ = req
	_ = rec

	// This test is a placeholder and should be completed as part of the modularisation work.
	// t.Fatalf("not implemented")
}
