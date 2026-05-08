package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type block struct {
	Type string `json:"type"`
	Text *mtext `json:"text,omitempty"`
}

type mtext struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type payload struct {
	Blocks []block `json:"blocks"`
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SendEventNotification posts an event notification to a Slack incoming webhook URL.
// Logs to stdout when webhookURL is empty (local dev / unconfigured).
func SendEventNotification(webhookURL, eventType, service, envName string) error {
	iconMap := map[string]string{
		"deploy": ":rocket:",
		"alert":  ":rotating_light:",
		"note":   ":memo:",
	}
	icon, ok := iconMap[eventType]
	if !ok {
		icon = ":pushpin:"
	}

	if webhookURL == "" {
		fmt.Printf("\n[SLACK] %s %s event on %s/%s\n\n", icon, capitalize(eventType), envName, service)
		return nil
	}

	msg := payload{
		Blocks: []block{
			{
				Type: "section",
				Text: &mtext{
					Type: "mrkdwn",
					Text: fmt.Sprintf(
						"%s *%s event* — `%s` on `%s`",
						icon, capitalize(eventType), service, envName,
					),
				},
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}
	return nil
}
