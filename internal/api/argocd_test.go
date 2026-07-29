package api

import (
	"encoding/json"
	"testing"

	"github.com/urbangeeks/kollaber/internal/store"
)

// The body produced by the notification template in our docs. Unlike the other
// webhooks this shape is ours, so this pins the contract we publish.
const sampleArgoCDDelivery = `{
  "app": "checkout",
  "type": "deploy",
  "revision": "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
  "sync_status": "Synced",
  "health_status": "Healthy",
  "operation_phase": "Succeeded",
  "project": "default",
  "namespace": "prod",
  "url": "https://argocd.example.com/applications/checkout",
  "message": "sync completed"
}`

func TestParseArgoCDPayload(t *testing.T) {
	var got argocdPayload
	if err := json.Unmarshal([]byte(sampleArgoCDDelivery), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.App != "checkout" {
		t.Errorf("app = %q", got.App)
	}
	if got.Revision != "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d" {
		t.Errorf("revision = %q", got.Revision)
	}
	if got.SyncStatus != "Synced" || got.HealthStatus != "Healthy" {
		t.Errorf("sync/health = %q/%q", got.SyncStatus, got.HealthStatus)
	}
	if got.OperationPhase != "Succeeded" {
		t.Errorf("operation_phase = %q", got.OperationPhase)
	}
	if got.Namespace != "prod" || got.Project != "default" {
		t.Errorf("namespace/project = %q/%q", got.Namespace, got.Project)
	}
}

// An Argo CD template renders an unset field as an empty string rather than
// omitting it, so a first-ever notification for an app that has never synced
// must still parse.
func TestArgoCDPayloadToleratesEmptyFields(t *testing.T) {
	var got argocdPayload
	body := `{"app":"checkout","type":"","revision":"","sync_status":"","health_status":"","operation_phase":""}`
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.App != "checkout" {
		t.Errorf("app = %q", got.App)
	}
}

func TestArgoCDEventStatus(t *testing.T) {
	tests := []struct {
		name   string
		phase  string
		health string
		want   string
	}{
		{"succeeded sync", "Succeeded", "Healthy", "success"},
		{"failed sync", "Failed", "Healthy", "failure"},
		{"errored sync", "Error", "Healthy", "failure"},
		{"running sync", "Running", "Progressing", "in_progress"},
		{"terminating sync", "Terminating", "Healthy", "in_progress"},
		{
			// The disagreement that matters: the sync did land, and that is the
			// change worth recording. Health stays in the metadata.
			name:   "sync succeeded onto a degraded app",
			phase:  "Succeeded",
			health: "Degraded",
			want:   "success",
		},
		{"health only, healthy", "", "Healthy", "success"},
		{"health only, degraded", "", "Degraded", "failure"},
		{"health only, missing", "", "Missing", "failure"},
		{"health only, progressing", "", "Progressing", "in_progress"},
		{"suspended is not a failure", "", "Suspended", "success"},
		{"neither field", "", "", "success"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := argocdEventStatus(tt.phase, tt.health); got != tt.want {
				t.Errorf("argocdEventStatus(%q, %q) = %q, want %q", tt.phase, tt.health, got, tt.want)
			}
		})
	}
}

// The type in the payload is what lets an on-app-deleted trigger record a
// teardown, so whatever the docs tell people to send has to be accepted.
func TestArgoCDDocumentedTypesAreValid(t *testing.T) {
	for _, eventType := range []string{"deploy", "teardown", "rollback"} {
		if !store.IsValidEventType(eventType) {
			t.Errorf("event type %q is documented for Argo CD but not valid", eventType)
		}
	}
}
