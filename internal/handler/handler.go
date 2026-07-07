package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"artifact/internal/artifact"
	"artifact/internal/service"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	svc *service.ArtifactService
}

const uploadUserHeader = "X-Upload-User"

func NewHandler(svc *service.ArtifactService) *Handler {
	return &Handler{svc: svc}
}

func NewRouter(svc *service.ArtifactService, apiKey string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = customHTTPErrorHandler

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
	for _, f := range files {
		src, err := f.Open()
		if err != nil {
			return artifact.NewRichError(artifact.ErrFileNotValid, "cannot read uploaded file", map[string]string{
				"file": f.Filename, "reason": "file_not_readable",
			})
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			return artifact.NewRichError(artifact.ErrFileNotValid, "cannot read uploaded file", map[string]string{
				"file": f.Filename, "reason": "file_not_readable",
			})
		}
		inputs = append(inputs, service.UploadInput{
			Filename:   f.Filename,
			Size:       f.Size,
			Reader:     bytes.NewReader(data),
			UploadUser: c.Request().Header.Get(uploadUserHeader),
		})
	}

	results, err := h.svc.UploadMultipleFiles(c.Request().Context(), inputs)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"artifacts": results})
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
