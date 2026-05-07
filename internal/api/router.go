package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"github.com/urbangeeks/kollaber/ui"
)

func NewRouter(q *store.Queries) *echo.Echo {
	e := echo.New()
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	origins := []string{"http://localhost:3000"}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		origins = strings.Split(v, ",")
	}
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: origins,
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAuthorization},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
	}))

	auth := NewAuthHandler(q)
	gh := NewGitHubAuthHandler(q)
	envs := NewEnvironmentsHandler(q)
	events := NewEventsHandler(q)
	comments := NewCommentsHandler(q)
	webhooks := NewWebhookHandler(q)
	admin := NewAdminHandler(q)
	invites := NewInviteHandler(q)
	services := NewServicesHandler(q)
	members := NewMembersHandler(q)

	e.GET("/health", func(c echo.Context) error { return c.JSON(200, echo.Map{"ok": true}) })

	e.POST("/auth/register", auth.Register)
	e.POST("/auth/login", auth.Login)
	e.GET("/auth/github", gh.Redirect)
	e.GET("/auth/github/callback", gh.Callback)

	authProtected := e.Group("/auth", middleware.Auth())
	authProtected.GET("/orgs", auth.ListOrgs)
	authProtected.POST("/switch", auth.SwitchOrg)
	authProtected.POST("/token", auth.GenerateCLIToken)
	authProtected.POST("/orgs", auth.CreateOrg)
	authProtected.PUT("/orgs/:id", auth.RenameOrg)
	e.GET("/invites/:token", invites.Get)
	e.POST("/invites/:token/accept", invites.Accept)

	e.POST("/webhooks/events", webhooks.Ingest)

	protected := e.Group("", middleware.Auth())
	protected.GET("/environments", envs.List)
	protected.POST("/environments", envs.Create)
	protected.PUT("/environments/:id", envs.Update)
	protected.DELETE("/environments/:id", envs.Delete)
	protected.POST("/events", events.Create)
	protected.GET("/events", events.List)
	protected.POST("/events/:id/comments", comments.Create)
	protected.GET("/events/:id/comments", comments.List)
	protected.GET("/services", services.List)
	protected.POST("/invites", invites.Create)
	protected.POST("/invites/:token/join", invites.Join)
	protected.GET("/members", members.List)
	protected.PATCH("/members/:userID", members.UpdateRole)
	protected.DELETE("/members/:userID", members.Remove)
	protected.GET("/members/invites", members.ListInvites)
	protected.DELETE("/members/invites/:token", members.RevokeInvite)

	adminGroup := e.Group("/admin", middleware.AdminOnly())
	adminGroup.GET("/orgs", admin.ListOrgs)

	// Serve embedded frontend — SPA fallback for Next.js App Router static export.
	// Next.js exports pages as path.html files, not path/index.html, so we must
	// resolve routes explicitly rather than relying on http.FileServer directory handling.
	staticFS, _ := fs.Sub(ui.FS, "dist")
	fileServer := http.FileServer(http.FS(staticFS))
	spaHandler := func(c echo.Context) error {
		path := strings.TrimPrefix(c.Request().URL.Path, "/")

		// 1. Exact file match (JS, CSS, images, fonts, etc.) — serve directly.
		if path != "" {
			if f, err := staticFS.Open(path); err == nil {
				stat, _ := f.Stat()
				f.Close()
				if stat != nil && !stat.IsDir() {
					fileServer.ServeHTTP(c.Response(), c.Request())
					return nil
				}
			}
		}

		// 2. Next.js page — try path.html (e.g. auth/callback → auth/callback.html).
		if path != "" {
			if _, err := staticFS.Open(path + ".html"); err == nil {
				c.Request().URL.Path = "/" + path + ".html"
				fileServer.ServeHTTP(c.Response(), c.Request())
				return nil
			}
		}

		// 3. Root / SPA fallback — use "/" not "/index.html" because http.FileServer
		// redirects /index.html → / which would cause an infinite loop.
		c.Request().URL.Path = "/"
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}
	e.GET("/", spaHandler)
	e.GET("/*", spaHandler)

	return e
}
