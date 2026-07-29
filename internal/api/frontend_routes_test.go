package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// browserAccept is what a browser sends on a top-level navigation.
const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"

// pages stands in for the exported Next.js build. Using a synthetic filesystem
// rather than the real embed keeps this deterministic: ui/dist is an untracked
// build artifact, so a test reading it would pass or fail depending on whether
// anyone had run the frontend build.
func pages() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<html>")},
		"decisions.html":              {Data: []byte("<html>")},
		"inventory.html":              {Data: []byte("<html>")},
		"search.html":                 {Data: []byte("<html>")},
		"incidents.html":              {Data: []byte("<html>")},
		"admin.html":                  {Data: []byte("<html>")},
		"settings/slack.html":         {Data: []byte("<html>")},
		"settings/notifications.html": {Data: []byte("<html>")},
		"auth/callback.html":          {Data: []byte("<html>")},
	}
}

func request(method, path, accept string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if accept != "" {
		r.Header.Set("Accept", accept)
	}
	return r
}

func TestFrontendPageFor(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		accept   string
		wantPage string
	}{
		// The bug: each of these is both a page and an authenticated endpoint,
		// and the endpoint was claiming the browser's navigation.
		{"decisions page", http.MethodGet, "/decisions", browserAccept, "decisions.html"},
		{"inventory page", http.MethodGet, "/inventory", browserAccept, "inventory.html"},
		{"search page", http.MethodGet, "/search", browserAccept, "search.html"},
		{"incidents page", http.MethodGet, "/incidents", browserAccept, "incidents.html"},
		{"nested settings page", http.MethodGet, "/settings/slack", browserAccept, "settings/slack.html"},
		{"admin page", http.MethodGet, "/admin", browserAccept, "admin.html"},

		// A query string is part of a shared link, not part of the path.
		{"page with query string", http.MethodGet, "/decisions?environment_id=abc", browserAccept, "decisions.html"},
		{"trailing slash", http.MethodGet, "/decisions/", browserAccept, "decisions.html"},

		// The app's own fetch and the CLI must still reach the API.
		{"fetch default accept", http.MethodGet, "/decisions", "*/*", ""},
		{"no accept header", http.MethodGet, "/decisions", "", ""},
		{"explicit json", http.MethodGet, "/decisions", "application/json", ""},
		{"server-sent events", http.MethodGet, "/events/stream", "text/event-stream", ""},

		// Writes are never a navigation.
		{"post is never diverted", http.MethodPost, "/incidents", browserAccept, ""},
		{"delete is never diverted", http.MethodDelete, "/freezes/abc", browserAccept, ""},

		// Endpoints with no page behind them stay endpoints.
		{"api-only path", http.MethodGet, "/events", browserAccept, ""},
		{"api-only nested path", http.MethodGet, "/metrics/dora", browserAccept, ""},

		// The OAuth callbacks are GETs a browser really does navigate to, and
		// diverting them would break sign-in. They have no exported page, which
		// is what keeps them safe.
		{"github oauth callback", http.MethodGet, "/auth/github/callback?code=x", browserAccept, ""},
		{"sso callback", http.MethodGet, "/auth/sso/callback?code=x", browserAccept, ""},
		// ...while the frontend's own callback page does have one.
		{"frontend auth callback page", http.MethodGet, "/auth/callback", browserAccept, "auth/callback.html"},

		// The root is the SPA handler's job already.
		{"root", http.MethodGet, "/", browserAccept, ""},
	}

	fsys := pages()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, ok := frontendPageFor(fsys, request(tt.method, tt.path, tt.accept))

			if tt.wantPage == "" {
				if ok {
					t.Errorf("diverted to %q, want the request to fall through to the API", page)
				}
				return
			}
			if !ok {
				t.Fatalf("not diverted, want %q", tt.wantPage)
			}
			if page != tt.wantPage {
				t.Errorf("page = %q, want %q", page, tt.wantPage)
			}
		})
	}
}

// A build that has not been run yet must degrade to the old behaviour rather
// than serving a blank page.
func TestFrontendPageForWithNoBuild(t *testing.T) {
	if _, ok := frontendPageFor(fstest.MapFS{}, request(http.MethodGet, "/decisions", browserAccept)); ok {
		t.Error("diverted with no exported page present")
	}
	if _, ok := frontendPageFor(nil, request(http.MethodGet, "/decisions", browserAccept)); ok {
		t.Error("diverted with a nil filesystem")
	}
}
