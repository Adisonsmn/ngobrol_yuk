package config

import (
	"os"
	"regexp"
	"strings"
)

// Validation helpers
func IsValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(strings.ToLower(email))
}

func IsValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	return usernameRegex.MatchString(username)
}

func SanitizeString(str string) string {
	// Remove leading/trailing whitespace and limit length
	str = strings.TrimSpace(str)
	if len(str) > 1000 {
		str = str[:1000]
	}
	return str
}

// Environment helpers
func GetEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func IsProduction() bool {
	return strings.ToLower(os.Getenv("ENVIRONMENT")) == "production"
}
