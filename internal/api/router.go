package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"github.com/urbangeeks/kollaber/ui"
)

func NewRouter(q *store.Queries, pool *pgxpool.Pool) *echo.Echo {
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

	staticFS, _ := fs.Sub(ui.FS, "dist")
	serve := func(c echo.Context, name string) error {
		http.ServeFileFS(c.Response(), c.Request(), staticFS, name)
		return nil
	}

	hub := NewHub()

	auth := NewAuthHandler(q, pool)
	gh := NewGitHubAuthHandler(q)
	otp := NewOTPHandler(q)
	envs := NewEnvironmentsHandler(q)
	events := NewEventsHandler(q, hub)
	comments := NewCommentsHandler(q, hub)
	webhooks := NewWebhookHandler(q, hub)
	alertmanager := NewAlertmanagerHandler(q, hub)
	terraform := NewTerraformHandler(q, hub)
	atlantis := NewAtlantisHandler(q, hub)
	argocd := NewArgoCDHandler(q, hub)
	streamH := NewStreamHandler(hub)
	admin := NewAdminHandler(q)
	invites := NewInviteHandler(q)
	services := NewServicesHandler(q)
	members := NewMembersHandler(q)
	notifications := NewNotificationsHandler(q)
	slackSettings := NewSlackSettingsHandler(q)
	teamsSettings := NewTeamsSettingsHandler(q)
	billingH := NewBillingHandler(q)
	aiH := NewAIHandler(q)
	agentH := NewAgentHandler(q)
	auditH := NewAuditHandler(q)
	ssoH := NewSSOHandler(q)
	incidents := NewIncidentsHandler(q)
	doraH := NewDORAHandler(q)
	searchH := NewSearchHandler(q)
	postmortemH := NewPostmortemHandler(q)
	annotationsH := NewAnnotationsHandler(q)
	decisionsH := NewDecisionsHandler(q)
	inventoryH := NewInventoryHandler(q)
	freezesH := NewFreezesHandler(q)

	e.GET("/health", func(c echo.Context) error { return c.JSON(200, echo.Map{"ok": true}) })

	e.POST("/auth/register", auth.Register)
	e.POST("/auth/login", auth.Login)
	e.POST("/auth/otp/send", otp.Send)
	e.POST("/auth/otp/verify", otp.Verify)
	e.GET("/auth/github", gh.Redirect)
	e.GET("/auth/github/callback", gh.Callback)
	e.GET("/auth/sso", ssoH.Initiate)
	e.GET("/auth/sso/callback", ssoH.Callback)

	authProtected := e.Group("/auth", middleware.Auth())
	authProtected.GET("/orgs", auth.ListOrgs)
	authProtected.POST("/switch", auth.SwitchOrg)
	authProtected.POST("/token", auth.GenerateCLIToken)
	authProtected.POST("/orgs", auth.CreateOrg)
	authProtected.PUT("/orgs/:id", auth.RenameOrg)
	e.GET("/invites/:token", invites.Get)
	e.POST("/invites/:token/accept", invites.Accept)

	// Explicit frontend routes that must never hit auth middleware
	e.GET("/auth/callback", func(c echo.Context) error {
		http.ServeFileFS(c.Response(), c.Request(), staticFS, "auth/callback.html")
		return nil
	})

	e.POST("/webhooks/events", webhooks.Ingest)
	e.POST("/webhooks/alertmanager", alertmanager.Ingest)
	e.POST("/webhooks/terraform", terraform.Ingest)
	e.POST("/webhooks/atlantis", atlantis.Ingest)
	e.POST("/webhooks/argocd", argocd.Ingest)

	protected := e.Group("", middleware.Auth())
	protected.GET("/environments", envs.List)
	protected.GET("/environments/stats", envs.Stats)
	protected.POST("/environments", envs.Create)
	protected.PUT("/environments/:id", envs.Update)
	protected.DELETE("/environments/:id", envs.Delete)
	protected.POST("/events", events.Create)
	protected.GET("/events", events.List)
	protected.GET("/events/:id", events.Get)
	protected.GET("/events/:id/suspects", events.Suspects)
	protected.GET("/events/stream", streamH.Stream)
	protected.POST("/events/:id/comments", comments.Create)
	protected.GET("/events/:id/comments", comments.List)
	protected.PATCH("/comments/:id", comments.SetDecision)
	protected.GET("/decisions", decisionsH.List)
	protected.GET("/inventory", inventoryH.List)
	protected.GET("/freezes", freezesH.List)
	protected.POST("/freezes", freezesH.Create)
	protected.DELETE("/freezes/:id", freezesH.Delete)
	protected.POST("/events/:id/summary", aiH.SummarizeEvent)
	protected.POST("/events/:id/postmortem", aiH.PostmortemEvent)
	protected.POST("/ai/chat", agentH.Chat)
	protected.POST("/incidents", incidents.Create)
	protected.GET("/incidents", incidents.List)
	protected.GET("/incidents/:id", incidents.Get)
	protected.PATCH("/incidents/:id", incidents.Update)
	protected.POST("/incidents/:id/events", incidents.AttachEvents)
	protected.POST("/incidents/:id/postmortem", aiH.PostmortemIncident)
	protected.GET("/metrics/dora", doraH.Metrics)
	protected.GET("/search", searchH.Search)
	protected.POST("/postmortems", postmortemH.Generate)
	protected.GET("/annotations", annotationsH.List)
	protected.POST("/annotations", annotationsH.Query)
	protected.GET("/services", services.List)
	protected.POST("/invites", invites.Create)
	protected.POST("/invites/:token/join", invites.Join)
	protected.GET("/members", members.List)
	protected.PATCH("/members/:userID", members.UpdateRole)
	protected.DELETE("/members/:userID", members.Remove)
	protected.GET("/members/invites", members.ListInvites)
	protected.DELETE("/members/invites/:token", members.RevokeInvite)
	protected.GET("/settings/notifications", notifications.Get)
	protected.PUT("/settings/notifications", notifications.Put)
	protected.GET("/settings/slack", slackSettings.Get)
	protected.PUT("/settings/slack", slackSettings.Put)
	protected.POST("/settings/slack/test", slackSettings.Test)
	protected.GET("/settings/teams", teamsSettings.Get)
	protected.PUT("/settings/teams", teamsSettings.Put)
	protected.POST("/settings/teams/test", teamsSettings.Test)
	protected.GET("/audit-logs", auditH.List)
	protected.GET("/settings/sso", ssoH.GetConfig)
	protected.PUT("/settings/sso", ssoH.SaveConfig)
	protected.GET("/billing", billingH.Get)
	protected.POST("/billing/checkout", billingH.Checkout)
	protected.POST("/billing/portal", billingH.Portal)
	e.POST("/webhooks/stripe", billingH.StripeWebhook)

	adminGroup := e.Group("/admin", middleware.AdminOnly())
	adminGroup.GET("/orgs", admin.ListOrgs)

	// Serve embedded frontend — SPA fallback for Next.js App Router static export.
	// Uses http.ServeFileFS (Go 1.22+) to serve files directly without the redirect
	// behaviour of http.FileServer (which redirects /index.html→/ and dirs→dir/).
	spaHandler := func(c echo.Context) error {
		path := strings.TrimPrefix(c.Request().URL.Path, "/")

		// 1. Exact file (JS, CSS, images, fonts, etc.) — serve directly.
		if path != "" {
			if f, err := staticFS.Open(path); err == nil {
				stat, _ := f.Stat()
				f.Close()
				if stat != nil && !stat.IsDir() {
					return serve(c, path)
				}
			}
		}

		// 2. Next.js exported page — try path.html (e.g. auth/callback.html).
		if path != "" {
			if _, err := staticFS.Open(path + ".html"); err == nil {
				return serve(c, path+".html")
			}
		}

		// 3. SPA fallback — serve root index.html for any unmatched route.
		return serve(c, "index.html")
	}
	e.GET("/", spaHandler)
	e.GET("/*", spaHandler)

	return e
}
