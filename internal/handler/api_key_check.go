package handler

import (
	"artifact/internal/artifact"

	"github.com/labstack/echo/v4"
)

func checkApiKey(expectedKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get("X-API-Key")
			if key == "" || key != expectedKey {
				return artifact.ErrAuthFailed
			}
			return next(c)
		}
	}
}
