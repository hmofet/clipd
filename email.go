package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// sendMagicLink sends a login email via the Resend API.
func sendMagicLink(to, token string) error {
	baseURL := os.Getenv("BASE_URL")
	resendDomain := os.Getenv("RESEND_DOMAIN")
	resendKey := os.Getenv("RESEND_API_KEY")

	link := fmt.Sprintf("%s/auth/verify?token=%s", baseURL, token)

	body := fmt.Sprintf(`{
		"from": "clipd <noreply@%s>",
		"to": ["%s"],
		"subject": "Your clipd login link",
		"html": "<p>Click to log in:</p><p><a href=\"%s\">Open clipd</a></p><p>This link expires in 15 minutes.</p>"
	}`, resendDomain, to, link)

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+resendKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("resend API returned %d", resp.StatusCode)
	}
	return nil
}
