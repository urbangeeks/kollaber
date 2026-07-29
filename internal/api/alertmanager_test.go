package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// A verbatim Alertmanager v4 delivery, used to pin the json tags: a typo in one
// would silently yield an empty field rather than a parse error.
const sampleDelivery = `{
  "version": "4",
  "groupKey": "{}:{alertname=\"HighErrorRate\"}",
  "truncatedAlerts": 0,
  "status": "firing",
  "receiver": "kollaber",
  "groupLabels": {"alertname": "HighErrorRate"},
  "commonLabels": {"alertname": "HighErrorRate", "severity": "critical"},
  "commonAnnotations": {"summary": "Error rate above 5%"},
  "externalURL": "http://alertmanager:9093",
  "alerts": [
    {
      "status": "firing",
      "labels": {"alertname": "HighErrorRate", "severity": "critical", "service": "checkout", "job": "checkout-api"},
      "annotations": {"summary": "Error rate above 5%", "description": "5xx rate is 12% over 5m"},
      "startsAt": "2026-07-22T10:31:00.000Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph?g0.expr=...",
      "fingerprint": "a1b2c3d4e5f6"
    }
  ]
}`

func TestParseAlertmanagerPayload(t *testing.T) {
	var got alertmanagerPayload
	if err := json.Unmarshal([]byte(sampleDelivery), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Status != "firing" {
		t.Errorf("status = %q, want firing", got.Status)
	}
	if len(got.Alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(got.Alerts))
	}

	a := got.Alerts[0]
	if a.Fingerprint != "a1b2c3d4e5f6" {
		t.Errorf("fingerprint = %q", a.Fingerprint)
	}
	if a.Labels["severity"] != "critical" {
		t.Errorf("severity = %q", a.Labels["severity"])
	}
	if a.Annotations["description"] != "5xx rate is 12% over 5m" {
		t.Errorf("description = %q", a.Annotations["description"])
	}
	if a.GeneratorURL == "" {
		t.Error("generatorURL did not parse")
	}
	want := time.Date(2026, 7, 22, 10, 31, 0, 0, time.UTC)
	if !a.StartsAt.Equal(want) {
		t.Errorf("startsAt = %v, want %v", a.StartsAt, want)
	}
	// Alertmanager sends the zero time for endsAt while an alert is firing.
	if !a.EndsAt.IsZero() {
		t.Errorf("endsAt = %v, want zero while firing", a.EndsAt)
	}
}

func TestResolveService(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"prefers service", map[string]string{"service": "checkout", "job": "checkout-api", "alertname": "HighErrorRate"}, "checkout"},
		{"falls back to job", map[string]string{"job": "checkout-api", "alertname": "HighErrorRate"}, "checkout-api"},
		{"falls back to app", map[string]string{"app": "checkout-web", "alertname": "HighErrorRate"}, "checkout-web"},
		{"falls back to alertname", map[string]string{"alertname": "HighErrorRate"}, "HighErrorRate"},
		{"blank labels are skipped", map[string]string{"service": "   ", "job": "checkout-api"}, "checkout-api"},
		{"no usable labels", map[string]string{"instance": "10.0.0.1:9090"}, "unknown"},
		{"nil labels", nil, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveService(tt.labels); got != tt.want {
				t.Errorf("resolveService() = %q, want %q", got, tt.want)
			}
		})
	}
}

func newSecretRequest(t *testing.T, headers map[string]string) echo.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return echo.New().NewContext(req, httptest.NewRecorder())
}

func TestCheckAlertmanagerSecret(t *testing.T) {
	const secret = "s3cr3t"
	body := []byte(`{"status":"firing"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name    string
		env     string
		headers map[string]string
		want    bool
	}{
		{"unset secret allows anything", "", nil, true},
		{"bearer token matches", secret, map[string]string{"Authorization": "Bearer " + secret}, true},
		{"bearer token mismatches", secret, map[string]string{"Authorization": "Bearer wrong"}, false},
		{"shared header matches", secret, map[string]string{"X-Kollaber-Secret": secret}, true},
		{"shared header mismatches", secret, map[string]string{"X-Kollaber-Secret": "wrong"}, false},
		{"valid hmac", secret, map[string]string{"X-Hub-Signature-256": validSig}, true},
		{"invalid hmac", secret, map[string]string{"X-Hub-Signature-256": "sha256=deadbeef"}, false},
		{"no credentials at all", secret, nil, false},
		{"basic auth is not accepted", secret, map[string]string{"Authorization": "Basic " + secret}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WEBHOOK_SECRET", tt.env)
			c := newSecretRequest(t, tt.headers)
			if got := checkWebhookSecret(c, body); got != tt.want {
				t.Errorf("checkWebhookSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
