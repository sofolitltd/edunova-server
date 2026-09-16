package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"edunova-server/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type fcmV1Message struct {
	Message      fcmV1Payload `json:"message"`
	ValidateOnly bool         `json:"validate_only,omitempty"`
}

type fcmV1Payload struct {
	Token        string            `json:"token,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *fcmAndroid       `json:"android,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroid struct {
	Priority string `json:"priority"`
}

var (
	cachedToken     *oauth2.Token
	cachedTokenFile string
	cachedTokenTime time.Time
)

func getAccessToken() (string, error) {
	saFile := config.AppConfig.FCMServiceAccount

	if cachedToken != nil && cachedTokenFile == saFile && time.Now().Before(cachedToken.Expiry.Add(-5*time.Minute)) {
		return cachedToken.AccessToken, nil
	}

	data, err := os.ReadFile(saFile)
	if err != nil {
		return "", fmt.Errorf("failed to read service account file %s: %w", saFile, err)
	}

	tokenSource, err := google.JWTAccessTokenSourceWithScope(data, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return "", fmt.Errorf("failed to create token source: %w", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	cachedToken = token
	cachedTokenFile = saFile
	cachedTokenTime = time.Now()

	return token.AccessToken, nil
}

// SendFCMV1 sends a push notification to a single token
func SendFCMV1(token string, title string, body string) error {
	return SendFCMV1WithData(token, title, body, nil)
}

// SendFCMV1WithData sends a push notification with data payload for deep linking
func SendFCMV1WithData(token string, title string, body string, data map[string]string) error {
	projectID := config.AppConfig.FCMProjectID
	if projectID == "" {
		return nil
	}

	accessToken, err := getAccessToken()
	if err != nil {
		return err
	}

	msg := fcmV1Message{
		Message: fcmV1Payload{
			Token: token,
			Notification: &fcmNotification{
				Title: title,
				Body:  body,
			},
			Data:    data,
			Android: &fcmAndroid{Priority: "high"},
		},
	}

	return sendV1Request(projectID, accessToken, msg)
}

// SendFCMTopicV1 sends a push notification to an FCM topic (single API call)
func SendFCMTopicV1(topic string, title string, body string) error {
	return SendFCMTopicV1WithData(topic, title, body, nil)
}

// SendFCMTopicV1WithData sends a topic notification with data payload for deep linking
func SendFCMTopicV1WithData(topic string, title string, body string, data map[string]string) error {
	projectID := config.AppConfig.FCMProjectID
	if projectID == "" {
		log.Printf("[FCM] No project ID, skipping topic send to %s", topic)
		return nil
	}

	accessToken, err := getAccessToken()
	if err != nil {
		log.Printf("[FCM] Auth error: %v", err)
		return err
	}

	msg := fcmV1Message{
		Message: fcmV1Payload{
			Topic: topic,
			Notification: &fcmNotification{
				Title: title,
				Body:  body,
			},
			Data:    data,
			Android: &fcmAndroid{Priority: "high"},
		},
	}

	if err := sendV1Request(projectID, accessToken, msg); err != nil {
		log.Printf("[FCM] Topic send to %s failed: %v", topic, err)
		return err
	}

	log.Printf("[FCM] Topic sent to %s", topic)
	return nil
}

// SendFCMV1ToToken sends to a single token (used for targeted notifications)
func SendFCMV1ToToken(token string, title string, body string) error {
	return SendFCMV1WithData(token, title, body, nil)
}

// SendFCMV1ToTokenWithData sends to a single token with data payload
func SendFCMV1ToTokenWithData(token string, title string, body string, data map[string]string) error {
	return SendFCMV1WithData(token, title, body, data)
}

func sendV1Request(projectID, accessToken string, msg fcmV1Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal FCM message: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create FCM request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("FCM request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FCM V1 API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
