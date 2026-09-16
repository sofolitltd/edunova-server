package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"edunova-server/config"
)

func SendSMS(mobile string, message string) error {
	apiKey := config.AppConfig.SMSAPIKey
	senderID := config.AppConfig.SMSSenderID
	apiURL := config.AppConfig.SMSAPIURL

	if apiKey == "" {
		log.Printf("[SMS] No API key configured, skipping SMS to %s: %s", mobile, message)
		return nil
	}

	normalizedMobile := normalizeMobile(mobile)

	params := url.Values{}
	params.Set("api_key", apiKey)
	params.Set("type", "text")
	params.Set("number", normalizedMobile)
	params.Set("senderid", senderID)
	params.Set("message", message)

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		log.Printf("[SMS] Failed to send to %s: %v", mobile, err)
		return fmt.Errorf("SMS request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[SMS] Failed to read response for %s: %v", mobile, err)
		return fmt.Errorf("SMS response read failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[SMS] API error for %s (status %d): %s", mobile, resp.StatusCode, string(body))
		return fmt.Errorf("SMS API returned status %d", resp.StatusCode)
	}

	log.Printf("[SMS] Sent to %s (status %d): %s", mobile, resp.StatusCode, string(body))
	return nil
}

func normalizeMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	mobile = strings.TrimPrefix(mobile, "+")
	if strings.HasPrefix(mobile, "0") {
		mobile = "88" + mobile
	}
	return mobile
}
