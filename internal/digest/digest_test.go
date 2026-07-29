package digest

import (
	"strings"
	"testing"
	"time"

	"github.com/urbangeeks/kollaber/internal/store"
)

// Every replica must agree on which week it is claiming, or two pods in
// different zones would claim different rows for the same week and mail the
// org twice.
func TestWeekStart(t *testing.T) {
	monday := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   time.Time
	}{
		{"monday midnight is its own week start", monday},
		{"monday midday", time.Date(2026, 7, 27, 12, 30, 0, 0, time.UTC)},
		{"wednesday", time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)},
		{"sunday is the end of the same week", time.Date(2026, 8, 2, 23, 59, 59, 0, time.UTC)},
		// Sunday is weekday 0 in Go, so a naive offset puts it in the week
		// after the one it belongs to.
		{"sunday just before midnight UTC", time.Date(2026, 8, 2, 23, 59, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WeekStart(tt.in); !got.Equal(monday) {
				t.Errorf("WeekStart(%s) = %s, want %s", tt.in, got, monday)
			}
		})
	}
}

// A timestamp in a zone ahead of UTC must not be pulled into the wrong week.
func TestWeekStartNormalizesZone(t *testing.T) {
	// Monday 08:00 in UTC+10 is Sunday 22:00 UTC — the previous week.
	tokyo := time.FixedZone("UTC+10", 10*60*60)
	got := WeekStart(time.Date(2026, 8, 3, 8, 0, 0, 0, tokyo))

	want := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("WeekStart = %s, want %s", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("week start is not UTC: %s", got.Location())
	}
}

func weekly() Weekly {
	return Weekly{
		OrgName:   "Acme",
		WeekStart: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		WeekEnd:   time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC),
		Environments: []store.DigestEnvironment{
			{Name: "prod", Deploys: 9, FailedDeploys: 1, Rollbacks: 1, Alerts: 3, Total: 14},
			{Name: "staging", Deploys: 4, Total: 4},
			{Name: "retired", Total: 0},
		},
		Threads: []store.DigestThread{
			{Type: "alert", Service: "checkout", EnvironmentName: "prod", Comments: 7},
		},
		IncidentsOpened:   1,
		IncidentsResolved: 2,
	}
}

func TestWeeklyTotals(t *testing.T) {
	w := weekly()

	if got := w.TotalEvents(); got != 18 {
		t.Errorf("TotalEvents() = %d, want 18", got)
	}
	if got := w.Deploys(); got != 13 {
		t.Errorf("Deploys() = %d, want 13", got)
	}
	if got := w.FailedDeploys(); got != 1 {
		t.Errorf("FailedDeploys() = %d, want 1", got)
	}
	if got := w.Alerts(); got != 3 {
		t.Errorf("Alerts() = %d, want 3", got)
	}
}

// A long-lived install accumulates environments nobody uses. Mailing a wall of
// zeroes every Monday is how a digest teaches people to filter it.
func TestActiveEnvironmentsDropsQuietOnes(t *testing.T) {
	got := weekly().ActiveEnvironments()

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, env := range got {
		if env.Name == "retired" {
			t.Error("an environment with no activity was included")
		}
	}
}

func TestQuiet(t *testing.T) {
	tests := []struct {
		name string
		w    Weekly
		want bool
	}{
		{"a busy week is not quiet", weekly(), false},
		{"nothing at all", Weekly{}, true},
		{
			name: "environments present but empty",
			w:    Weekly{Environments: []store.DigestEnvironment{{Name: "prod", Total: 0}}},
			want: true,
		},
		{
			name: "an incident alone is worth mailing",
			w:    Weekly{IncidentsOpened: 1},
			want: false,
		},
		{
			// Discussion on an older event is exactly the thing someone would
			// otherwise miss, even in a week that shipped nothing.
			name: "discussion alone is worth mailing",
			w:    Weekly{Threads: []store.DigestThread{{Type: "alert", Comments: 3}}},
			want: false,
		},
		{
			name: "a resolved incident alone is worth mailing",
			w:    Weekly{IncidentsResolved: 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.Quiet(); got != tt.want {
				t.Errorf("Quiet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubjectLeadsWithWhatMatters(t *testing.T) {
	tests := []struct {
		name string
		w    Weekly
		want string
	}{
		{
			name: "incidents win",
			w:    Weekly{OrgName: "Acme", WeekStart: weekly().WeekStart, Environments: []store.DigestEnvironment{{Deploys: 9}}, IncidentsOpened: 2},
			want: "[Kollaber] Acme: 9 deploys, 2 incidents — week of Jul 20",
		},
		{
			name: "failures next",
			w:    Weekly{OrgName: "Acme", WeekStart: weekly().WeekStart, Environments: []store.DigestEnvironment{{Deploys: 9, FailedDeploys: 2}}},
			want: "[Kollaber] Acme: 9 deploys, 2 failed — week of Jul 20",
		},
		{
			name: "quiet weeks just count deploys",
			w:    Weekly{OrgName: "Acme", WeekStart: weekly().WeekStart, Environments: []store.DigestEnvironment{{Deploys: 9}}},
			want: "[Kollaber] Acme: 9 deploys — week of Jul 20",
		},
		{
			name: "singular deploy",
			w:    Weekly{OrgName: "Acme", WeekStart: weekly().WeekStart, Environments: []store.DigestEnvironment{{Deploys: 1}}},
			want: "[Kollaber] Acme: 1 deploy — week of Jul 20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.Subject(); got != tt.want {
				t.Errorf("Subject() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestEnvSummary(t *testing.T) {
	tests := []struct {
		name string
		env  store.DigestEnvironment
		want string
	}{
		{"deploys only", store.DigestEnvironment{Deploys: 3, Total: 3}, "3 deploys"},
		{"singular", store.DigestEnvironment{Deploys: 1, Total: 1}, "1 deploy"},
		{"failures are called out", store.DigestEnvironment{Deploys: 9, FailedDeploys: 1, Total: 9}, "9 deploys (1 failed)"},
		{
			name: "everything",
			env:  store.DigestEnvironment{Deploys: 9, FailedDeploys: 1, Rollbacks: 2, Alerts: 3, Total: 14},
			want: "9 deploys (1 failed) · 2 rollbacks · 3 alerts",
		},
		// A zero carries no information, so it does not earn a clause.
		{"zeroes are omitted", store.DigestEnvironment{Alerts: 2, Total: 2}, "2 alerts"},
		{
			// Notes and scales are counted in the total but not broken out.
			name: "types the digest does not name fall back to the total",
			env:  store.DigestEnvironment{Total: 5},
			want: "5 events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envSummary(tt.env); got != tt.want {
				t.Errorf("envSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Service and environment names are org-controlled text that lands in an email
// body. Anything less than escaping them makes the digest an injection vector
// into every subscriber's inbox.
func TestHTMLEscapesOrgControlledText(t *testing.T) {
	w := Weekly{
		OrgName:   `Acme <script>alert(1)</script>`,
		WeekStart: weekly().WeekStart,
		WeekEnd:   weekly().WeekEnd,
		Environments: []store.DigestEnvironment{
			{Name: `prod"><img src=x onerror=alert(1)>`, Deploys: 1, Total: 1},
		},
		Threads: []store.DigestThread{
			{Type: "alert", Service: `<b>checkout</b>`, EnvironmentName: "prod", Comments: 2},
		},
	}

	got := w.HTML()

	if strings.Contains(got, "<script>") {
		t.Error("org name was not escaped")
	}
	// The payload text survives escaping as inert characters; what must not
	// survive is a tag that opens. Assert on the construct, not the substring.
	if strings.Contains(got, "<img") {
		t.Error("environment name was not escaped: an img tag opens")
	}
	if strings.Contains(got, "<b>checkout</b>") {
		t.Error("service name was not escaped")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Error("expected the escaped form to be present")
	}
}

func TestHTMLIncludesTheNumbers(t *testing.T) {
	got := weekly().HTML()

	for _, want := range []string{"prod", "staging", "13", "checkout", "Open the timeline"} {
		if !strings.Contains(got, want) {
			t.Errorf("digest HTML is missing %q", want)
		}
	}
	// Quiet environments are dropped, so the retired one must not appear.
	if strings.Contains(got, "retired") {
		t.Error("an environment with no activity was rendered")
	}
}

func TestAppURLFallsBackToHosted(t *testing.T) {
	t.Setenv("FRONTEND_URL", "")
	if got := appURL(); got != "https://kollaber.io" {
		t.Errorf("appURL() = %q", got)
	}

	t.Setenv("FRONTEND_URL", "https://kollaber.internal/")
	if got := appURL(); got != "https://kollaber.internal" {
		t.Errorf("appURL() = %q, want the trailing slash trimmed", got)
	}
}
