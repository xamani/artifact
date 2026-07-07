package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"artifact/internal/artifact"

	"github.com/labstack/echo/v4"
)

func TestCheckApiKey_missing(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = customHTTPErrorHandler
	e.Use(checkApiKey("secret"))
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCheckApiKey_ok(t *testing.T) {
	e := echo.New()
	e.Use(checkApiKey("secret"))
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestMapError_notFound(t *testing.T) {
	err := mapErrorToHTTP(artifact.ErrArtifactNotFound)
	if err.Code != http.StatusNotFound {
		t.Fatalf("code = %d", err.Code)
	}
}

func TestMapError_validation(t *testing.T) {
	err := mapErrorToHTTP(artifact.NewRichError(artifact.ErrFileNotValid, "extension not allowed", map[string]string{
		"reason": "extension_not_allowed", "extension": ".exe",
	}))
	if err.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", err.Code)
	}
	body, ok := err.Message.(ErrorBody)
	if !ok {
		t.Fatalf("message type %T", err.Message)
	}
	if body.Details["extension"] != ".exe" {
		t.Fatalf("details = %v", body.Details)
	}
}
