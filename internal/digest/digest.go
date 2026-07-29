// Package digest builds and sends the weekly per-org recap.
//
// The scheduler lives in-process rather than in a cron job so that a digest
// arrives on a plain `docker run` install with nothing configured. Correctness
// under more than one replica comes from the claim in digest_sends, not from
// there being only one scheduler.
package digest

import (
	"context"
	"time"

	"github.com/urbangeeks/kollaber/internal/store"
)

// Weekly is everything one org's email is rendered from. Assembled separately
// from the sending so it can be tested without a mail provider.
type Weekly struct {
	OrgName           string
	WeekStart         time.Time
	WeekEnd           time.Time
	Environments      []store.DigestEnvironment
	Threads           []store.DigestThread
	IncidentsOpened   int64
	IncidentsResolved int64
}

// maxThreads caps the discussion list. A digest is a nudge, not a report: past
// a handful of threads nobody reads further, and the link to the timeline is
// right there.
const maxThreads = 5

// TotalEvents is the whole week's activity across every environment.
func (w Weekly) TotalEvents() int64 {
	var n int64
	for _, env := range w.Environments {
		n += env.Total
	}
	return n
}

// Deploys is the week's deploy count across every environment.
func (w Weekly) Deploys() int64 {
	var n int64
	for _, env := range w.Environments {
		n += env.Deploys
	}
	return n
}

// FailedDeploys is the week's failed deploys across every environment.
func (w Weekly) FailedDeploys() int64 {
	var n int64
	for _, env := range w.Environments {
		n += env.FailedDeploys
	}
	return n
}

// Alerts is the week's alert count across every environment.
func (w Weekly) Alerts() int64 {
	var n int64
	for _, env := range w.Environments {
		n += env.Alerts
	}
	return n
}

// Quiet reports whether nothing happened at all.
//
// A digest for a week with no events, no incidents and no discussion is a mail
// that costs attention and returns none, so the caller skips it. An org that
// genuinely went quiet learns nothing from being told so every Monday.
func (w Weekly) Quiet() bool {
	return w.TotalEvents() == 0 && w.IncidentsOpened == 0 && w.IncidentsResolved == 0 && len(w.Threads) == 0
}

// ActiveEnvironments drops the environments that saw nothing, so a long-lived
// install with a dozen retired environments does not mail a wall of zeroes.
// When every environment is quiet the caller has already skipped the send.
func (w Weekly) ActiveEnvironments() []store.DigestEnvironment {
	out := make([]store.DigestEnvironment, 0, len(w.Environments))
	for _, env := range w.Environments {
		if env.Total > 0 {
			out = append(out, env)
		}
	}
	return out
}

// Build assembles one org's digest for the window.
func Build(ctx context.Context, q *store.Queries, org store.DigestOrg, from, to time.Time) (Weekly, error) {
	envs, err := q.DigestEnvironments(ctx, org.ID, from, to)
	if err != nil {
		return Weekly{}, err
	}
	threads, err := q.DigestThreads(ctx, org.ID, from, to, maxThreads)
	if err != nil {
		return Weekly{}, err
	}
	opened, resolved, err := q.DigestIncidents(ctx, org.ID, from, to)
	if err != nil {
		return Weekly{}, err
	}

	return Weekly{
		OrgName:           org.Name,
		WeekStart:         from,
		WeekEnd:           to,
		Environments:      envs,
		Threads:           threads,
		IncidentsOpened:   opened,
		IncidentsResolved: resolved,
	}, nil
}

// WeekStart returns the Monday 00:00 UTC that begins the week containing t.
//
// UTC rather than a local zone because an org's members are not in one place,
// and the boundary has to be the same instant for whichever replica evaluates
// it — two pods in different zones must agree on which week they are claiming.
func WeekStart(t time.Time) time.Time {
	t = t.UTC()
	// time.Weekday puts Sunday at 0; the ISO week starts on Monday.
	offset := (int(t.Weekday()) + 6) % 7
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}
