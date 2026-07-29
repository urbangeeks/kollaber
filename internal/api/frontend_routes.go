package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// frontendPageFor reports the exported page a request should be answered with,
// when the request is a browser navigating to a path that has one.
//
// It exists because a page and an endpoint may share a name — /decisions,
// /search, /incidents, /settings/slack — and Echo matches on path alone, so the
// endpoint claims the path and anyone refreshing the page or opening a shared
// link gets a 401 and a JSON body where the page should be. In-app navigation
// hides this completely: Next.js routes those clicks client-side and never asks
// the server, so only a hard navigation is ever broken — which is exactly what
// a "send this link to your incident thread" feature produces.
//
// Two conditions keep this from swallowing the API:
//
//   - The request must be a GET that explicitly accepts HTML. The app's own
//     fetch() sends Accept: */* and the CLI sends none, so neither is diverted,
//     and no write ever is.
//   - An exported page must actually exist for the path. A browser pointed at
//     /events still reaches the API because there is no events.html, and the
//     OAuth callbacks — /auth/github/callback, /auth/sso/callback, which a
//     browser really does navigate to — keep working for the same reason.
func frontendPageFor(pages fs.FS, r *http.Request) (string, bool) {
	if pages == nil || r.Method != http.MethodGet {
		return "", false
	}
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return "", false
	}

	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		// The root is already the SPA's; leave it to the normal handler.
		return "", false
	}

	page := path + ".html"
	f, err := pages.Open(page)
	if err != nil {
		return "", false
	}
	f.Close()
	return page, true
}
