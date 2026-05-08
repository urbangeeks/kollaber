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

// SendEventNotification emails all opted-in org members about a new event.
// Silently logs to stdout when RESEND_API_KEY is unset (local dev).
func SendEventNotification(recipients []string, eventType, service, envName string) error {
	if len(recipients) == 0 {
		return nil
	}

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("\n[NOTIFY] %s event on %s/%s → %s\n\n", eventType, envName, service, strings.Join(recipients, ", "))
		return nil
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "Kollaber <noreply@kollaber.io>"
	}

	payload := emailPayload{
		From:    from,
		To:      recipients,
		Subject: fmt.Sprintf("[Kollaber] %s event: %s on %s", capitalize(eventType), service, envName),
		HTML:    eventNotificationHTML(eventType, service, envName),
	}

	body, err := json.Marshal(payload)
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
		var e struct{ Message string `json:"message"` }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("resend: %s", e.Message)
	}
	return nil
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
