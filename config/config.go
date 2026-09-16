package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	Port              string
	JWTSecret         string
	SMSAPIKey         string
	SMSSenderID       string
	SMSAPIURL         string
	FCMServerKey      string
	FCMServiceAccount string
	FCMProjectID      string
}

var AppConfig Config

func Load() {
	godotenv.Load()

	AppConfig = Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		Port:              getEnv("PORT", "8080"),
		JWTSecret:         getEnv("JWT_SECRET", "edu-nova-secret"),
		SMSAPIKey:         os.Getenv("SMS_API_KEY"),
		SMSSenderID:       getEnv("SMS_SENDER_ID", "8809617624898"),
		SMSAPIURL:         getEnv("SMS_API_URL", "http://bulksmsbd.net/api/smsapi"),
		FCMServerKey:      os.Getenv("FCM_SERVER_KEY"),
		FCMServiceAccount: getEnv("FCM_SERVICE_ACCOUNT", "serviceAccountKey.json"),
		FCMProjectID:      getEnv("FCM_PROJECT_ID", "edu-nova-bd"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
