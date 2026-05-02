package api

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

func NewRouter(q *store.Queries) *echo.Echo {
	e := echo.New()
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
	}))

	auth := NewAuthHandler(q)
	envs := NewEnvironmentsHandler(q)
	events := NewEventsHandler(q)
	comments := NewCommentsHandler(q)
	webhooks := NewWebhookHandler(q)

	e.POST("/auth/register", auth.Register)
	e.POST("/auth/login", auth.Login)

	e.POST("/webhooks/events", webhooks.Ingest)

	protected := e.Group("", middleware.Auth())
	protected.GET("/environments", envs.List)
	protected.POST("/environments", envs.Create)
	protected.POST("/events", events.Create)
	protected.GET("/events", events.List)
	protected.POST("/events/:id/comments", comments.Create)
	protected.GET("/events/:id/comments", comments.List)

	return e
}
