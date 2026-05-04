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
	gh := NewGitHubAuthHandler(q)
	envs := NewEnvironmentsHandler(q)
	events := NewEventsHandler(q)
	comments := NewCommentsHandler(q)
	webhooks := NewWebhookHandler(q)
	admin := NewAdminHandler(q)
	invites := NewInviteHandler(q)

	e.POST("/auth/register", auth.Register)
	e.POST("/auth/login", auth.Login)
	e.GET("/auth/github", gh.Redirect)
	e.GET("/auth/github/callback", gh.Callback)

	authProtected := e.Group("/auth", middleware.Auth())
	authProtected.GET("/orgs", auth.ListOrgs)
	authProtected.POST("/switch", auth.SwitchOrg)
	e.GET("/invites/:token", invites.Get)
	e.POST("/invites/:token/accept", invites.Accept)

	e.POST("/webhooks/events", webhooks.Ingest)

	protected := e.Group("", middleware.Auth())
	protected.GET("/environments", envs.List)
	protected.POST("/environments", envs.Create)
	protected.POST("/events", events.Create)
	protected.GET("/events", events.List)
	protected.POST("/events/:id/comments", comments.Create)
	protected.GET("/events/:id/comments", comments.List)
	protected.POST("/invites", invites.Create)
	protected.POST("/invites/:token/join", invites.Join)

	adminGroup := e.Group("/admin", middleware.AdminOnly())
	adminGroup.GET("/orgs", admin.ListOrgs)

	return e
}
