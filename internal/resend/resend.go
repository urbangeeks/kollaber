package resend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type emailPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendOTP sends a one-time code to the given address.
// If RESEND_API_KEY is unset it logs to stdout instead (local dev).
func SendOTP(to, code string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("\n[OTP] %s → code: %s\n\n", to, code)
		return nil
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "Kollaber <noreply@kollaber.io>"
	}

	payload := emailPayload{
		From:    from,
		To:      []string{to},
		Subject: "Your Kollaber login code",
		HTML:    otpHTML(code),
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

func otpHTML(code string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;background:#0a0a0a;color:#fff;padding:40px 20px">
  <div style="max-width:400px;margin:0 auto">
    <h2 style="margin-bottom:8px">Your Kollaber login code</h2>
    <p style="color:#999;margin-bottom:32px">Enter this code to sign in. It expires in 10 minutes.</p>
    <div style="background:#1a1a1a;border:1px solid #333;border-radius:8px;padding:24px;text-align:center">
      <span style="font-size:36px;font-weight:700;letter-spacing:8px;font-family:monospace">%s</span>
    </div>
    <p style="color:#666;font-size:13px;margin-top:24px">
      If you didn't request this code, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`, code)
}
