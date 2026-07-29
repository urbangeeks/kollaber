package digest

import (
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/urbangeeks/kollaber/internal/store"
)

// appURL is where "open the timeline" points. FRONTEND_URL is what the rest of
// the codebase already uses for outbound links, so a self-hosted install that
// has configured OAuth and billing links has configured this one too.
func appURL() string {
	if v := strings.TrimSpace(os.Getenv("FRONTEND_URL")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return "https://kollaber.io"
}

// Subject names the week and leads with the number that matters. A subject line
// of "Your weekly summary" is indistinguishable from every other product's, and
// gets filtered accordingly.
func (w Weekly) Subject() string {
	week := w.WeekStart.Format("Jan 2")
	switch {
	case w.IncidentsOpened > 0:
		return fmt.Sprintf("[Kollaber] %s: %d %s, %d %s — week of %s",
			w.OrgName,
			w.Deploys(), plural(w.Deploys(), "deploy", "deploys"),
			w.IncidentsOpened, plural(w.IncidentsOpened, "incident", "incidents"),
			week)
	case w.FailedDeploys() > 0:
		return fmt.Sprintf("[Kollaber] %s: %d %s, %d failed — week of %s",
			w.OrgName,
			w.Deploys(), plural(w.Deploys(), "deploy", "deploys"),
			w.FailedDeploys(), week)
	default:
		return fmt.Sprintf("[Kollaber] %s: %d %s — week of %s",
			w.OrgName,
			w.Deploys(), plural(w.Deploys(), "deploy", "deploys"), week)
	}
}

// DevLine is the one-line stdout form used when no mail provider is configured.
func (w Weekly) DevLine() string {
	return fmt.Sprintf("Weekly digest for %s (week of %s): %d events, %d deploys, %d alerts",
		w.OrgName, w.WeekStart.Format("2006-01-02"), w.TotalEvents(), w.Deploys(), w.Alerts())
}

// HTML renders the email.
//
// Inline styles and a table-free layout on purpose: mail clients strip <style>
// blocks, and every value interpolated here is org-controlled text — service
// names, environment names — so all of it goes through html.EscapeString.
func (w Weekly) HTML() string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#0a0a0a;color:#fff;padding:40px 20px">
  <div style="max-width:560px;margin:0 auto">`)

	fmt.Fprintf(&b, `
    <h2 style="margin-bottom:4px">📈 Week of %s</h2>
    <p style="color:#999;margin-top:0;margin-bottom:24px">%s · %s → %s</p>`,
		esc(w.WeekStart.Format("January 2, 2006")),
		esc(w.OrgName),
		esc(w.WeekStart.Format("Jan 2")),
		esc(w.WeekEnd.AddDate(0, 0, -1).Format("Jan 2")))

	// Headline numbers.
	b.WriteString(`
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:20px;margin-bottom:16px">`)
	fmt.Fprintf(&b, `
      <p style="margin:0 0 8px"><span style="color:#666">Deploys:</span> <strong>%d</strong>%s</p>
      <p style="margin:0 0 8px"><span style="color:#666">Alerts:</span> <strong>%d</strong></p>
      <p style="margin:0"><span style="color:#666">Incidents:</span> <strong>%d opened</strong>, %d resolved</p>`,
		w.Deploys(), failedSuffix(w.FailedDeploys()),
		w.Alerts(),
		w.IncidentsOpened, w.IncidentsResolved)
	b.WriteString(`
    </div>`)

	// Per environment.
	if envs := w.ActiveEnvironments(); len(envs) > 0 {
		b.WriteString(`
    <h3 style="font-size:15px;margin:24px 0 8px">By environment</h3>
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:20px">`)
		for i, env := range envs {
			margin := "0 0 12px"
			if i == len(envs)-1 {
				margin = "0"
			}
			fmt.Fprintf(&b, `
      <p style="margin:%s"><strong>%s</strong><br>
        <span style="color:#999;font-size:13px">%s</span></p>`,
				margin, esc(env.Name), esc(envSummary(env)))
		}
		b.WriteString(`
    </div>`)
	}

	// Discussion.
	if len(w.Threads) > 0 {
		b.WriteString(`
    <h3 style="font-size:15px;margin:24px 0 8px">Most discussed</h3>
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:20px">`)
		for i, t := range w.Threads {
			margin := "0 0 12px"
			if i == len(w.Threads)-1 {
				margin = "0"
			}
			label := t.Type
			if t.Service != "" {
				label += " · " + t.Service
			}
			fmt.Fprintf(&b, `
      <p style="margin:%s"><strong>%s</strong><br>
        <span style="color:#999;font-size:13px">%s · %d %s</span></p>`,
				margin, esc(label), esc(t.EnvironmentName),
				t.Comments, plural(t.Comments, "comment", "comments"))
		}
		b.WriteString(`
    </div>`)
	}

	fmt.Fprintf(&b, `
    <p style="margin-top:24px">
      <a href="%s/dashboard" style="color:#a78bfa">Open the timeline →</a>
    </p>
    <p style="color:#666;font-size:13px;margin-top:24px">
      You are receiving this because you enabled the weekly digest in Kollaber settings.
    </p>
  </div>
</body>
</html>`, esc(appURL()))

	return b.String()
}

// envSummary is the one-line breakdown under an environment's name. Only the
// counts that are non-zero appear: "3 deploys" reads better than
// "3 deploys, 0 rollbacks, 0 alerts", and a zero carries no information a
// reader needs.
func envSummary(env store.DigestEnvironment) string {
	parts := make([]string, 0, 4)
	if env.Deploys > 0 {
		s := fmt.Sprintf("%d %s", env.Deploys, plural(env.Deploys, "deploy", "deploys"))
		if env.FailedDeploys > 0 {
			s += fmt.Sprintf(" (%d failed)", env.FailedDeploys)
		}
		parts = append(parts, s)
	}
	if env.Rollbacks > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", env.Rollbacks, plural(env.Rollbacks, "rollback", "rollbacks")))
	}
	if env.Alerts > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", env.Alerts, plural(env.Alerts, "alert", "alerts")))
	}
	if len(parts) == 0 {
		// Reached only when the week's events were all types the digest does
		// not break out by name.
		return fmt.Sprintf("%d %s", env.Total, plural(env.Total, "event", "events"))
	}
	return strings.Join(parts, " · ")
}

func failedSuffix(failed int64) string {
	if failed == 0 {
		return ""
	}
	return fmt.Sprintf(` <span style="color:#f87171">(%d failed)</span>`, failed)
}

func plural(n int64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func esc(s string) string { return html.EscapeString(s) }
