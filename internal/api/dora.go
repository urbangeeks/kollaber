package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type DORAHandler struct{ q *store.Queries }

func NewDORAHandler(q *store.Queries) *DORAHandler { return &DORAHandler{q} }

// DORA performance tiers, per the Accelerate / State of DevOps bands. Stored as
// a plain string so the UI and CLI can colour-code without re-deriving them.
const (
	ratingElite  = "elite"
	ratingHigh   = "high"
	ratingMedium = "medium"
	ratingLow    = "low"
	ratingNA     = "n/a"
)

type doraMetric struct {
	// Value is the metric's natural unit: deploys/day, a 0-1 ratio, or seconds.
	Value   float64 `json:"value"`
	Display string  `json:"display"`
	Rating  string  `json:"rating"`
	// Samples is how many data points the value is based on; 0 means n/a.
	Samples int64 `json:"samples"`
}

type doraResponse struct {
	WindowDays      int        `json:"window_days"`
	Since           string     `json:"since"`
	EnvironmentID   string     `json:"environment_id,omitempty"`
	DeployFrequency doraMetric `json:"deploy_frequency"`
	LeadTime        doraMetric `json:"lead_time"`
	ChangeFailRate  doraMetric `json:"change_failure_rate"`
	TimeToRestore   doraMetric `json:"time_to_restore"`
	// TimeToRestoreScope is always "org": incidents have no environment, so MTTR
	// covers the whole org even when environment_id narrows the other metrics.
	TimeToRestoreScope string              `json:"time_to_restore_scope"`
	Trend              []doraTrendPoint    `json:"trend"`
	RestoreTrend       []restoreTrendPoint `json:"restore_trend"`
}

type doraTrendPoint struct {
	Day         string  `json:"day"`
	Deploys     int64   `json:"deploys"`
	Failed      int64   `json:"failed"`
	LeadSeconds float64 `json:"lead_seconds"`
}

type restoreTrendPoint struct {
	Day     string  `json:"day"`
	Seconds float64 `json:"seconds"`
}

// Metrics handles GET /metrics/dora?days=30&environment_id=uuid.
func (h *DORAHandler) Metrics(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	days := parseInt32(c.QueryParam("days"), 30, 365)
	if days < 1 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -int(days))

	params := store.DORAParams{
		OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
		Since: pgtype.Timestamptz{Time: since, Valid: true},
	}

	resp := doraResponse{
		WindowDays:         int(days),
		Since:              since.UTC().Format(time.RFC3339),
		TimeToRestoreScope: "org",
	}

	if envStr := c.QueryParam("environment_id"); envStr != "" {
		envID, err := uuid.Parse(envStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id must be a uuid"})
		}
		pg := pgtype.UUID{Bytes: envID, Valid: true}
		params.EnvironmentID = &pg
		resp.EnvironmentID = envID.String()
	}

	ctx := c.Request().Context()
	m, err := h.q.DORA(ctx, params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not compute metrics"})
	}
	trend, err := h.q.DeployTrend(ctx, params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not compute trend"})
	}
	restoreTrend, err := h.q.RestoreTrend(ctx, params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not compute restore trend"})
	}

	resp.DeployFrequency = deployFrequencyMetric(m.TotalDeploys, int(days))
	resp.LeadTime = leadTimeMetric(m.LeadTimeSeconds, m.LeadTimeSamples)
	resp.ChangeFailRate = changeFailureMetric(m.FailedDeploys, m.TotalDeploys)
	resp.TimeToRestore = restoreMetric(m.RestoreSeconds, m.ResolvedIncidents)

	resp.Trend = make([]doraTrendPoint, 0, len(trend))
	for _, p := range trend {
		resp.Trend = append(resp.Trend, doraTrendPoint{
			Day:         p.Day.Time.UTC().Format("2006-01-02"),
			Deploys:     p.Deploys,
			Failed:      p.Failed,
			LeadSeconds: p.LeadSeconds,
		})
	}

	resp.RestoreTrend = make([]restoreTrendPoint, 0, len(restoreTrend))
	for _, p := range restoreTrend {
		resp.RestoreTrend = append(resp.RestoreTrend, restoreTrendPoint{
			Day:     p.Day.Time.UTC().Format("2006-01-02"),
			Seconds: p.Seconds,
		})
	}

	return c.JSON(http.StatusOK, resp)
}

func deployFrequencyMetric(total int64, days int) doraMetric {
	perDay := float64(total) / float64(days)
	m := doraMetric{Value: perDay, Samples: total}
	switch {
	case total == 0:
		m.Rating, m.Display = ratingLow, "no deploys"
	case perDay >= 1:
		m.Rating = ratingElite
	case perDay >= 1.0/7.0:
		m.Rating = ratingHigh
	case perDay >= 1.0/30.0:
		m.Rating = ratingMedium
	default:
		m.Rating = ratingLow
	}
	if m.Display == "" {
		m.Display = strconv.FormatFloat(perDay, 'f', 2, 64) + "/day"
	}
	return m
}

func leadTimeMetric(seconds float64, samples int64) doraMetric {
	if samples == 0 {
		// No commit metadata on any deploy in the window — can't compute.
		return doraMetric{Rating: ratingNA, Display: "no commit data", Samples: 0}
	}
	m := doraMetric{Value: seconds, Samples: samples, Display: humanDuration(seconds)}
	switch {
	case seconds < day:
		m.Rating = ratingElite
	case seconds < week:
		m.Rating = ratingHigh
	case seconds < month:
		m.Rating = ratingMedium
	default:
		m.Rating = ratingLow
	}
	return m
}

func changeFailureMetric(failed, total int64) doraMetric {
	if total == 0 {
		return doraMetric{Rating: ratingNA, Display: "no deploys", Samples: 0}
	}
	rate := float64(failed) / float64(total)
	m := doraMetric{Value: rate, Samples: total,
		Display: strconv.FormatFloat(rate*100, 'f', 1, 64) + "%"}
	switch {
	case rate <= 0.05:
		m.Rating = ratingElite
	case rate <= 0.10:
		m.Rating = ratingHigh
	case rate <= 0.15:
		m.Rating = ratingMedium
	default:
		m.Rating = ratingLow
	}
	return m
}

func restoreMetric(seconds float64, samples int64) doraMetric {
	if samples == 0 {
		return doraMetric{Rating: ratingNA, Display: "no incidents", Samples: 0}
	}
	m := doraMetric{Value: seconds, Samples: samples, Display: humanDuration(seconds)}
	switch {
	case seconds < hour:
		m.Rating = ratingElite
	case seconds < day:
		m.Rating = ratingHigh
	case seconds < week:
		m.Rating = ratingMedium
	default:
		m.Rating = ratingLow
	}
	return m
}

const (
	hour  = 3600.0
	day   = 24 * hour
	week  = 7 * day
	month = 30 * day
)

// humanDuration renders a span of seconds as a compact, human-readable string
// (e.g. "3.2h", "1.5d"), picking the largest sensible unit.
func humanDuration(seconds float64) string {
	switch {
	case seconds < 60:
		return strconv.FormatFloat(seconds, 'f', 0, 64) + "s"
	case seconds < hour:
		return strconv.FormatFloat(seconds/60, 'f', 1, 64) + "m"
	case seconds < day:
		return strconv.FormatFloat(seconds/hour, 'f', 1, 64) + "h"
	default:
		return strconv.FormatFloat(seconds/day, 'f', 1, 64) + "d"
	}
}
