package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL          string
	Port                 string
	JWTSecret            string
	SMSAPIKey            string
	SMSSenderID          string
	SMSAPIURL            string
	FCMServerKey         string
	FCMServiceAccount    string
	FCMProjectID         string
	S3Endpoint           string
	S3AccessKeyID        string
	S3SecretAccessKey    string
	S3Region             string
	S3Bucket             string
	S3PublicBaseURL      string
	InvoiceIssuerName    string
	InvoiceIssuerAddress string
	InvoiceIssuerPhone   string
}

var AppConfig Config

func Load() {
	godotenv.Load()

	AppConfig = Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		Port:                 getEnv("PORT", "8080"),
		JWTSecret:            getEnv("JWT_SECRET", "edu-nova-secret"),
		SMSAPIKey:            os.Getenv("SMS_API_KEY"),
		SMSSenderID:          getEnv("SMS_SENDER_ID", "8809617624898"),
		SMSAPIURL:            getEnv("SMS_API_URL", "http://bulksmsbd.net/api/smsapi"),
		FCMServerKey:         os.Getenv("FCM_SERVER_KEY"),
		FCMServiceAccount:    getEnv("FCM_SERVICE_ACCOUNT", "serviceAccountKey.json"),
		FCMProjectID:         getEnv("FCM_PROJECT_ID", "edu-nova-bd"),
		S3Endpoint:           firstEnv("S3_ENDPOINT", "AWS_ENDPOINT_URL_S3"),
		S3AccessKeyID:        firstEnv("S3_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID"),
		S3SecretAccessKey:    firstEnv("S3_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY"),
		S3Region:             getEnv("S3_REGION", getEnv("AWS_REGION", "auto")),
		S3Bucket:             getEnv("S3_BUCKET", "assets"),
		S3PublicBaseURL:      os.Getenv("S3_PUBLIC_BASE_URL"),
		InvoiceIssuerName:    getEnv("INVOICE_ISSUER_NAME", "EduNova"),
		InvoiceIssuerAddress: os.Getenv("INVOICE_ISSUER_ADDRESS"),
		InvoiceIssuerPhone:   os.Getenv("INVOICE_ISSUER_PHONE"),
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if val := os.Getenv(k); val != "" {
			return val
		}
	}
	return ""
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
