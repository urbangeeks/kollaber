package resend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SendHTML delivers one already-rendered email to every recipient, or prints
// devLine to stdout when RESEND_API_KEY is unset (local dev and unconfigured
// self-hosted installs).
//
// This is the single path to the mail provider. Each caller supplies its own
// subject and markup; nothing about a particular kind of notification lives
// here, so a new one cannot arrive with its own subtly different retry or
// error handling.
func SendHTML(recipients []string, subject, html, devLine string) error {
	if len(recipients) == 0 {
		return nil
	}

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("\n[NOTIFY] %s → %s\n\n", devLine, strings.Join(recipients, ", "))
		return nil
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "Kollaber <noreply@kollaber.io>"
	}

	body, err := json.Marshal(emailPayload{
		From:    from,
		To:      recipients,
		Subject: subject,
		HTML:    html,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("resend: %s", e.Message)
	}
	return nil
}

// SendEventNotification emails all opted-in org members about a new event.
func SendEventNotification(recipients []string, eventType, service, envName string) error {
	return SendHTML(
		recipients,
		fmt.Sprintf("[Kollaber] %s event: %s on %s", capitalize(eventType), service, envName),
		eventNotificationHTML(eventType, service, envName),
		fmt.Sprintf("%s event on %s/%s", eventType, envName, service),
	)
}

// SendIncidentNotification emails opted-in org members about an incident update.
// verb describes what happened, e.g. "opened", "mitigated", "resolved".
func SendIncidentNotification(recipients []string, title, severity, verb string) error {
	return SendHTML(
		recipients,
		fmt.Sprintf("[Kollaber] Incident %s: %s (%s)", verb, title, strings.ToUpper(severity)),
		incidentNotificationHTML(title, severity, verb),
		fmt.Sprintf("Incident %s (%s): %s", verb, strings.ToUpper(severity), title),
	)
}

func incidentNotificationHTML(title, severity, verb string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#0a0a0a;color:#fff;padding:40px 20px">
  <div style="max-width:480px;margin:0 auto">
    <h2 style="margin-bottom:8px">🚨 Incident %s</h2>
    <p style="color:#999;margin-bottom:24px">An incident was updated in your Kollaber organisation.</p>
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:20px">
      <p style="margin:0 0 8px"><span style="color:#666">Title:</span> <strong>%s</strong></p>
      <p style="margin:0"><span style="color:#666">Severity:</span> <strong>%s</strong></p>
    </div>
    <p style="color:#666;font-size:13px;margin-top:24px">
      You are receiving this because you enabled incident notifications in Kollaber settings.
    </p>
  </div>
</body>
</html>`, verb, title, strings.ToUpper(severity))
}

func eventNotificationHTML(eventType, service, envName string) string {
	iconMap := map[string]string{
		"deploy": "🚀",
		"alert":  "🚨",
		"note":   "📝",
	}
	icon := iconMap[eventType]
	if icon == "" {
		icon = "📌"
	}
	label := capitalize(eventType)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#0a0a0a;color:#fff;padding:40px 20px">
  <div style="max-width:480px;margin:0 auto">
    <h2 style="margin-bottom:8px">%s %s event</h2>
    <p style="color:#999;margin-bottom:24px">A new event was recorded in your Kollaber timeline.</p>
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:20px">
      <p style="margin:0 0 8px"><span style="color:#666">Service:</span> <strong>%s</strong></p>
      <p style="margin:0"><span style="color:#666">Environment:</span> <strong>%s</strong></p>
    </div>
    <p style="color:#666;font-size:13px;margin-top:24px">
      You are receiving this because you enabled %s notifications in Kollaber settings.
    </p>
  </div>
</body>
</html>`, icon, label, service, envName, label)
}
