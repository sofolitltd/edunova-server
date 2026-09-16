package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"edunova-server/config"
)

type AdminJWTClaims struct {
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
}

func GenerateAdminToken(email string) (string, error) {
	claims := AdminJWTClaims{
		Email: email,
		Exp:   time.Now().Add(24 * time.Hour).Unix(),
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)

	signingInput := header + "." + payloadEncoded
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret+"-admin"))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

func ValidateAdminToken(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(config.AppConfig.JWTSecret+"-admin"))
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return "", false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}

	var claims AdminJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", false
	}

	if time.Now().Unix() > claims.Exp {
		return "", false
	}

	return claims.Email, true
}
