package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"edunova-server/config"
)

type SMSBalance struct {
	Balance      float64 `json:"balance"`
	IsSuccess    bool    `json:"is_success"`
	StatusMsg    string  `json:"status_message"`
	ResponseCode int     `json:"response_code"`
}

func GetSMSBalance() (*SMSBalance, error) {
	apiKey := config.AppConfig.SMSAPIKey
	if apiKey == "" {
		return nil, fmt.Errorf("SMS API key not configured")
	}

	balanceURL := "https://bulksmsbd.net/api/getBalanceApi?api_key=" + url.QueryEscape(apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(balanceURL)
	if err != nil {
		return nil, fmt.Errorf("balance request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("balance response read failed: %w", err)
	}

	var balance SMSBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("balance response parse failed: %w", err)
	}

	return &balance, nil
}

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

	// BulkSMS BD always responds with HTTP 200, even on failure, and reports the
	// real outcome in the JSON body via response_code (202 == submitted).
	var result struct {
		ResponseCode int    `json:"response_code"`
		ErrorMessage string `json:"error_message"`
		SuccessMsg   string `json:"success_message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[SMS] Failed to parse response for %s: %v (body: %s)", mobile, err, string(body))
		return fmt.Errorf("SMS response parse failed: %w", err)
	}

	if result.ResponseCode != 202 {
		errMsg := result.ErrorMessage
		if errMsg == "" {
			errMsg = string(body)
		}
		log.Printf("[SMS] API rejected message to %s (code %d): %s", mobile, result.ResponseCode, errMsg)
		return fmt.Errorf("SMS failed (code %d): %s", result.ResponseCode, errMsg)
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
