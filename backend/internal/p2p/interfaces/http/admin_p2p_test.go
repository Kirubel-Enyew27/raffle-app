package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raffle-app/backend/internal/p2p/application"
	p2prepo "github.com/raffle-app/backend/internal/p2p/interfaces/repository"
)

func TestHTTP_AdminP2PRoutesRequireAuthentication(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := application.NewP2PService(
		p2prepo.NewGameRepo(db),
		p2prepo.NewMatchRepo(db),
		p2prepo.NewPlayerRepo(db),
		p2prepo.NewMoveRepo(db),
		p2prepo.NewSettingsRepo(db),
		p2prepo.NewEscrowRepo(db, ""),
		nil,
		nil,
	)
	router := setupTestRouter(svc)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/p2p/settings"},
		{http.MethodPatch, "/api/v1/admin/p2p/settings"},
		{http.MethodGet, "/api/v1/admin/p2p/matches"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401 for an anonymous caller, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
