package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
)

// requireAdmin allows owner and admin roles; blocks member and viewer.
func requireAdmin(c echo.Context) error {
	role, _ := c.Get(middleware.RoleKey).(string)
	if role != "owner" && role != "admin" {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "only admins can perform this action"})
	}
	return nil
}

// requireMember allows owner, admin, and member roles; blocks viewer.
func requireMember(c echo.Context) error {
	role, _ := c.Get(middleware.RoleKey).(string)
	if role == "viewer" || role == "" {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "viewers cannot create or modify content"})
	}
	return nil
}
