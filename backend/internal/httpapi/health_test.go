package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		dbErr      error
		wantStatus int
		wantBody   healthResponse
	}{
		{"database up", nil, http.StatusOK, healthResponse{Status: "ok", Database: "ok", Version: "test"}},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, healthResponse{Status: "degraded", Database: "unreachable", Version: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(Deps{DB: fakeDB{err: tt.dbErr}, Version: "test"})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var got healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got != tt.wantBody {
				t.Fatalf("body = %+v, want %+v", got, tt.wantBody)
			}
		})
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	router := NewRouter(Deps{DB: fakeDB{}, Version: "test"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Error.Code)
	}
}
