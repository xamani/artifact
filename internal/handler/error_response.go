package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"artifact/internal/artifact"

	"github.com/labstack/echo/v4"
)

type ErrorBody struct {
	Details map[string]string `json:"details,omitempty"`
	Error   string            `json:"error"`
	Message string            `json:"message"`
}

func buildErrorBody(err error, code, defaultMessage string) ErrorBody {
	body := ErrorBody{
		Error:   code,
		Message: defaultMessage,
	}
	if rich, ok := artifact.AsRichError(err); ok {
		if rich.Message != "" {
			body.Message = rich.Message
		} else {
			body.Message = err.Error()
		}
		if len(rich.Details) > 0 {
			body.Details = rich.Details
		}
	} else if err != nil {
		body.Message = err.Error()
	}
	return body
}

func mapErrorToHTTP(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, artifact.ErrAuthFailed):
		return echo.NewHTTPError(http.StatusUnauthorized, buildErrorBody(err, "authentication_failed", "authentication failed"))
	case errors.Is(err, artifact.ErrArtifactNotFound):
		return echo.NewHTTPError(http.StatusNotFound, buildErrorBody(err, "not_found", "artifact not found"))
	case errors.Is(err, artifact.ErrFileNotValid):
		return echo.NewHTTPError(http.StatusBadRequest, buildErrorBody(err, "validation_failed", "file validation failed"))
	case errors.Is(err, artifact.ErrBatchTooManyFiles):
		return echo.NewHTTPError(http.StatusBadRequest, buildErrorBody(err, "validation_failed", "batch upload limit exceeded"))
	case errors.Is(err, artifact.ErrStorageConnect):
		return echo.NewHTTPError(http.StatusServiceUnavailable, buildErrorBody(err, "storage_unavailable", "storage is not available"))
	default:
		slog.Error("request failed", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, ErrorBody{
			Error:   "internal_error",
			Message: "request failed",
		})
	}
}
