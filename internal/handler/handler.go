package handler

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"artifact/internal/artifact"
	"artifact/internal/service"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	svc *service.ArtifactService
}

const uploadUserHeader = "X-Upload-User"
const requestIDHeader = "X-Request-ID"

func NewHandler(svc *service.ArtifactService) *Handler {
	return &Handler{svc: svc}
}

func NewRouter(svc *service.ArtifactService, apiKey string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = customHTTPErrorHandler
	e.Use(recoverPanic)
	e.Use(logRequest)

	h := NewHandler(svc)

	e.GET("/health", h.HandleHealthCheck)

	v1 := e.Group("/api/v1", checkApiKey(apiKey))
	art := v1.Group("/artifacts")
	art.POST("/upload", h.HandleUploadFile)
	art.POST("/batch-upload", h.HandleUploadBatch)
	art.GET("/list", h.HandleGetArtifactList)
	art.GET("/*/metadata", h.HandleGetMetadata)
	art.DELETE("/*", h.HandleDeleteFile)
	art.GET("/*", h.HandleDownloadFile)

	return e
}

func recoverPanic(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered", "panic", recovered)
				err = echo.NewHTTPError(http.StatusInternalServerError, ErrorBody{
					Error:   "internal_error",
					Message: "request failed",
				})
			}
		}()
		return next(c)
	}
}

func logRequest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		requestID := c.Request().Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Response().Header().Set(requestIDHeader, requestID)

		err := next(c)
		if err != nil {
			c.Error(err)
		}
		slog.Info("request",
			"request_id", requestID,
			"method", c.Request().Method,
			"uri", c.Request().URL.RequestURI(),
			"status", c.Response().Status,
			"latency", time.Since(start),
			"error", err,
		)
		return nil
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func customHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	if he, ok := err.(*echo.HTTPError); ok {
		_ = c.JSON(he.Code, he.Message)
		return
	}

	httpErr := mapErrorToHTTP(err)
	_ = c.JSON(httpErr.Code, httpErr.Message)
}

func (h *Handler) HandleHealthCheck(c echo.Context) error {
	if err := h.svc.CheckStorageHealth(c.Request().Context()); err != nil {
		httpErr := mapErrorToHTTP(err)
		return c.JSON(httpErr.Code, httpErr.Message)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) HandleUploadFile(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return artifact.NewRichError(artifact.ErrFileNotValid, "multipart field file is required", map[string]string{
			"field": "file", "reason": "missing_file",
		})
	}

	src, err := file.Open()
	if err != nil {
		return artifact.NewRichError(artifact.ErrFileNotValid, "cannot read uploaded file", map[string]string{
			"field": "file", "reason": "file_not_readable",
		})
	}
	defer src.Close()

	result, err := h.svc.UploadArtifactFile(c.Request().Context(), service.UploadInput{
		Filename:   file.Filename,
		Size:       file.Size,
		Reader:     src,
		UploadUser: c.Request().Header.Get(uploadUserHeader),
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, result)
}

func (h *Handler) HandleUploadBatch(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return artifact.NewRichError(artifact.ErrFileNotValid, "invalid multipart form", map[string]string{
			"reason": "invalid_multipart",
		})
	}

	files := form.File["file"]
	if len(files) == 0 {
		files = form.File["files"]
	}
	if len(files) == 0 {
		return artifact.NewRichError(artifact.ErrFileNotValid, "no files in request", map[string]string{
			"reason": "missing_files", "fields": "file,files",
		})
	}

	inputs := make([]service.UploadInput, 0, len(files))
	closers := make([]io.Closer, 0, len(files))
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()
	for _, f := range files {
		src, err := f.Open()
		if err != nil {
			return artifact.NewRichError(artifact.ErrFileNotValid, "cannot read uploaded file", map[string]string{
				"file": f.Filename, "reason": "file_not_readable",
			})
		}
		closers = append(closers, src)
		inputs = append(inputs, service.UploadInput{
			Filename:   f.Filename,
			Size:       f.Size,
			Reader:     src,
			UploadUser: c.Request().Header.Get(uploadUserHeader),
		})
	}

	results, err := h.svc.UploadMultipleFiles(c.Request().Context(), inputs)
	if err != nil {
		return err
	}
	status := http.StatusCreated
	if len(results.Errors) > 0 {
		status = http.StatusMultiStatus
	}
	return c.JSON(status, results)
}

func (h *Handler) HandleGetArtifactList(c echo.Context) error {
	prefix := c.QueryParam("prefix")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	items, err := h.svc.GetArtifactList(c.Request().Context(), prefix, limit)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"artifacts": items})
}

func objectKeyFromParam(raw string) string {
	key := strings.TrimPrefix(raw, "/")
	key = strings.TrimSuffix(key, "/metadata")
	return artifact.NormalizeObjectKey(key)
}

func (h *Handler) HandleDownloadFile(c echo.Context) error {
	key := objectKeyFromParam(c.Param("*"))
	reader, meta, err := h.svc.DownloadArtifactFile(c.Request().Context(), key)
	if err != nil {
		return err
	}
	defer reader.Close()

	if name := meta["original-filename"]; name != "" {
		c.Response().Header().Set("Content-Disposition", attachmentHeader(name))
	}
	contentType := meta["mime-type"]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Response().Header().Set("Content-Type", contentType)
	return c.Stream(http.StatusOK, contentType, reader)
}

func attachmentHeader(filename string) string {
	name := filepath.Base(filename)
	return `attachment; filename="` + strings.ReplaceAll(name, `"`, "") + `"; filename*=UTF-8''` + url.PathEscape(name)
}

func (h *Handler) HandleGetMetadata(c echo.Context) error {
	key := objectKeyFromParam(c.Param("*"))
	meta, err := h.svc.GetArtifactMetadata(c.Request().Context(), key)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, meta)
}

func (h *Handler) HandleDeleteFile(c echo.Context) error {
	key := objectKeyFromParam(c.Param("*"))
	if err := h.svc.DeleteArtifactFile(c.Request().Context(), key); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
