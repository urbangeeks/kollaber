package teams

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type fact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type section struct {
	Facts []fact `json:"facts"`
}

type messageCard struct {
	Type       string    `json:"@type"`
	Context    string    `json:"@context"`
	ThemeColor string    `json:"themeColor"`
	Summary    string    `json:"summary"`
	Title      string    `json:"title"`
	Sections   []section `json:"sections"`
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SendEventNotification posts an event notification to a Teams incoming webhook URL.
// Logs to stdout when webhookURL is empty (local dev / unconfigured).
func SendEventNotification(webhookURL, eventType, service, envName string) error {
	colorMap := map[string]string{
		"deploy": "0078D4",
		"alert":  "D83B01",
		"note":   "7B83EB",
	}
	color, ok := colorMap[eventType]
	if !ok {
		color = "666666"
	}

	label := capitalize(eventType)
	summary := fmt.Sprintf("%s event: %s on %s", label, service, envName)

	if webhookURL == "" {
		fmt.Printf("\n[TEAMS] %s\n\n", summary)
		return nil
	}

	msg := messageCard{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: color,
		Summary:    summary,
		Title:      fmt.Sprintf("Kollaber — %s Event", label),
		Sections: []section{
			{
				Facts: []fact{
					{Name: "Type", Value: label},
					{Name: "Service", Value: service},
					{Name: "Environment", Value: envName},
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
		return fmt.Errorf("teams: unexpected status %d", resp.StatusCode)
	}
	return nil
}
