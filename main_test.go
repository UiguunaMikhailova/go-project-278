package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := newRouter()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("код ответа = %d, ожидался %d", rec.Code, http.StatusOK)
	}

	if body := rec.Body.String(); body != "pong" {
		t.Errorf("тело ответа = %q, ожидалось %q", body, "pong")
	}
}
