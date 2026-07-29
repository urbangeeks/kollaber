package digest

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/urbangeeks/kollaber/internal/store"
)

// Sender delivers one org's digest. Injected so the scheduler can be tested
// without a mail provider.
type Sender func(recipients []string, w Weekly) error

// defaultSendHour is the UTC hour on Monday at which the previous week's digest
// goes out. Mid-morning in the US and late afternoon in Europe: the digest is a
// prompt to look at last week, which is not a thing to receive at 3am.
const defaultSendHour = 14

// tickInterval is how often the scheduler looks for work. Hourly rather than
// weekly so a pod that restarts on Monday afternoon still sends, instead of
// waiting for the next timer that a redeploy keeps resetting.
const tickInterval = time.Hour

// Scheduler sends each org's weekly digest once.
type Scheduler struct {
	q        *store.Queries
	send     Sender
	now      func() time.Time
	sendHour int
}

// NewScheduler builds a scheduler. DIGEST_SEND_HOUR overrides the UTC hour on
// Monday at which digests go out.
func NewScheduler(q *store.Queries, send Sender) *Scheduler {
	hour := defaultSendHour
	if v := os.Getenv("DIGEST_SEND_HOUR"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 && parsed <= 23 {
			hour = parsed
		}
	}
	return &Scheduler{q: q, send: send, now: time.Now, sendHour: hour}
}

// Start runs the scheduler until ctx is cancelled. DIGEST_DISABLED=true turns
// it off, which is what a self-hoster who wants no outbound mail at all sets.
func (s *Scheduler) Start(ctx context.Context) {
	if os.Getenv("DIGEST_DISABLED") == "true" {
		log.Println("weekly digest disabled")
		return
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	// Run immediately so a deploy on Monday afternoon does not wait an hour.
	s.RunOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce sends any digest that is due and unclaimed, and returns how many it
// delivered. Safe to call as often as the ticker likes: the claim in
// digest_sends is what makes a repeat a no-op.
func (s *Scheduler) RunOnce(ctx context.Context) int {
	now := s.now().UTC()
	thisWeek := WeekStart(now)

	// The digest covers the week that has finished, and goes out once this
	// week's send time has passed. A pod that starts on Wednesday still sends
	// Monday's digest, because the condition is "past the hour", not "at it".
	if now.Before(thisWeek.Add(time.Duration(s.sendHour) * time.Hour)) {
		return 0
	}
	from := thisWeek.AddDate(0, 0, -7)
	to := thisWeek

	orgs, err := s.q.ListDigestOrgs(ctx)
	if err != nil {
		log.Printf("digest: could not list orgs: %v", err)
		return 0
	}

	var sent int
	for _, org := range orgs {
		if s.runOrg(ctx, org, from, to) {
			sent++
		}
	}
	return sent
}

// runOrg claims, builds and sends one org's digest, reporting whether mail went
// out. A claim that is kept without sending is deliberate: it means this week
// was considered and found not worth mailing, and rebuilding that answer every
// hour for the rest of the week would be pure load.
func (s *Scheduler) runOrg(ctx context.Context, org store.DigestOrg, from, to time.Time) bool {
	claimed, err := s.q.ClaimWeeklyDigest(ctx, org.ID, from)
	if err != nil {
		log.Printf("digest: could not claim %s: %v", org.Name, err)
		return false
	}
	if !claimed {
		return false
	}

	weekly, err := Build(ctx, s.q, org, from, to)
	if err != nil {
		log.Printf("digest: could not build %s: %v", org.Name, err)
		s.release(ctx, org, from)
		return false
	}
	if weekly.Quiet() {
		return false
	}

	recipients, err := s.q.ListDigestRecipients(ctx, org.ID)
	if err != nil {
		log.Printf("digest: could not load recipients for %s: %v", org.Name, err)
		s.release(ctx, org, from)
		return false
	}
	if len(recipients) == 0 {
		// Everyone unsubscribed between the org list and here.
		return false
	}

	if err := s.send(recipients, weekly); err != nil {
		// The send is a single call, so a reported failure means nothing was
		// delivered. Dropping the claim costs a small chance of a duplicate if
		// the provider actually accepted it, and buys back a week that would
		// otherwise be silently skipped.
		log.Printf("digest: could not send %s: %v", org.Name, err)
		s.release(ctx, org, from)
		return false
	}

	if err := s.q.RecordDigestRecipients(ctx, org.ID, from, len(recipients)); err != nil {
		log.Printf("digest: could not record recipients for %s: %v", org.Name, err)
	}
	return true
}

func (s *Scheduler) release(ctx context.Context, org store.DigestOrg, weekStart time.Time) {
	if err := s.q.ReleaseWeeklyDigest(ctx, org.ID, weekStart); err != nil {
		log.Printf("digest: could not release claim for %s: %v", org.Name, err)
	}
}
