package api

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// A verbatim HCP Terraform generic run notification, used to pin the json tags:
// a typo in one would silently yield an empty field rather than a parse error.
const sampleTerraformDelivery = `{
  "payload_version": 1,
  "notification_configuration_id": "nc-AeUQ2zfKZzW9TiGZ",
  "run_url": "https://app.terraform.io/app/acme-org/my-workspace/runs/run-FwnENkvDnrpyFC7M",
  "run_id": "run-FwnENkvDnrpyFC7M",
  "run_message": "Add five new queue workers",
  "run_created_at": "2019-01-25T18:34:00.000Z",
  "run_created_by": "sample-user",
  "workspace_id": "ws-XdeUVMWShTesDMME",
  "workspace_name": "my-workspace",
  "organization_name": "acme-org",
  "notifications": [
    {
      "message": "Run Canceled",
      "trigger": "run:errored",
      "run_status": "canceled",
      "run_updated_at": "2019-01-25T18:37:04.000Z",
      "run_updated_by": "sample-user"
    }
  ]
}`

func TestParseTerraformPayload(t *testing.T) {
	var got terraformPayload
	if err := json.Unmarshal([]byte(sampleTerraformDelivery), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.RunID != "run-FwnENkvDnrpyFC7M" {
		t.Errorf("run_id = %q", got.RunID)
	}
	if got.WorkspaceName != "my-workspace" {
		t.Errorf("workspace_name = %q", got.WorkspaceName)
	}
	if got.OrganizationName != "acme-org" {
		t.Errorf("organization_name = %q", got.OrganizationName)
	}
	if got.RunMessage != "Add five new queue workers" {
		t.Errorf("run_message = %q", got.RunMessage)
	}
	if got.RunURL == "" {
		t.Error("run_url did not parse")
	}
	if len(got.Notifications) != 1 {
		t.Fatalf("len(notifications) = %d, want 1", len(got.Notifications))
	}

	n := got.Notifications[0]
	if n.Trigger != "run:errored" {
		t.Errorf("trigger = %q", n.Trigger)
	}
	if n.RunStatus != "canceled" {
		t.Errorf("run_status = %q", n.RunStatus)
	}
	if n.RunUpdatedBy != "sample-user" {
		t.Errorf("run_updated_by = %q", n.RunUpdatedBy)
	}
	want := time.Date(2019, 1, 25, 18, 37, 4, 0, time.UTC)
	if !n.RunUpdatedAt.Equal(want) {
		t.Errorf("run_updated_at = %v, want %v", n.RunUpdatedAt, want)
	}
}

// A plan is not a change. Only a run that reached the infrastructure earns an
// event, or the timeline fills with markers for runs that touched nothing and
// DORA counts them as deployments.
func TestTerraformEventStatus(t *testing.T) {
	tests := []struct {
		runStatus  string
		wantStatus string
		wantKeep   bool
	}{
		{"applied", "success", true},
		{"errored", "failure", true},
		{"canceled", "", false},
		{"discarded", "", false},
		{"planning", "", false},
		{"planned", "", false},
		{"planned_and_finished", "", false},
		{"pending", "", false},
		{"applying", "", false},
		{"cost_estimating", "", false},
		{"policy_checking", "", false},
		// The verification payload sent when a notification config is saved
		// carries no run status at all.
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.runStatus, func(t *testing.T) {
			status, keep := terraformEventStatus(tt.runStatus)
			if keep != tt.wantKeep {
				t.Fatalf("keep = %v, want %v", keep, tt.wantKeep)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
		})
	}
}

// The sample delivery is a canceled run, so a real payload must produce no
// events rather than a deploy nobody made.
func TestSampleTerraformDeliveryIsSkipped(t *testing.T) {
	var payload terraformPayload
	if err := json.Unmarshal([]byte(sampleTerraformDelivery), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, keep := terraformEventStatus(payload.Notifications[0].RunStatus); keep {
		t.Error("a canceled run would be recorded as a deploy")
	}
}

func TestCheckTerraformSignature(t *testing.T) {
	const secret = "s3cr3t"
	body := []byte(`{"run_id":"run-1"}`)

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name    string
		env     string
		headers map[string]string
		want    bool
	}{
		{"unset secret allows anything", "", nil, true},
		{"valid sha512 signature", secret, map[string]string{"X-TFE-Notification-Signature": validSig}, true},
		{"wrong signature", secret, map[string]string{"X-TFE-Notification-Signature": "deadbeef"}, false},
		{"signature is not hex", secret, map[string]string{"X-TFE-Notification-Signature": "not-hex!"}, false},
		// Terraform sends the digest bare. A prefixed value is the SHA-256
		// convention from the other webhooks and must not verify here.
		{"sha256-style prefix is rejected", secret, map[string]string{"X-TFE-Notification-Signature": "sha512=" + validSig}, false},
		// Terraform Enterprise installs that leave the token unset send no
		// signature at all, so the shared-secret paths still have to work.
		{"falls back to bearer token", secret, map[string]string{"Authorization": "Bearer " + secret}, true},
		{"falls back to shared header", secret, map[string]string{"X-Kollaber-Secret": secret}, true},
		{"no credentials at all", secret, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WEBHOOK_SECRET", tt.env)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/terraform", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			c := echo.New().NewContext(req, httptest.NewRecorder())

			if got := checkTerraformSignature(c, body); got != tt.want {
				t.Errorf("checkTerraformSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A SHA-256 signature over the same body must not pass the SHA-512 check —
// the two webhooks use different algorithms and confusing them would accept a
// digest an attacker could compute against the weaker documented path.
func TestTerraformRejectsSHA256Digest(t *testing.T) {
	const secret = "s3cr3t"
	body := []byte(`{"run_id":"run-1"}`)
	t.Setenv("WEBHOOK_SECRET", secret)

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	sha512Sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/terraform", nil)
	// A digest of the right shape but the wrong length for SHA-512.
	req.Header.Set("X-TFE-Notification-Signature", sha512Sig[:64])
	c := echo.New().NewContext(req, httptest.NewRecorder())

	if checkTerraformSignature(c, body) {
		t.Error("a truncated digest verified")
	}
}
