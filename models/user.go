package models

import (
	"regexp"
	"strings"
	"time"
)

type User struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	Username  string    `bson:"username" json:"username"`
	Email     string    `bson:"email" json:"email"`
	Password  string    `bson:"password" json:"-"` // Hide password in JSON
	Bio       string    `bson:"bio" json:"bio"`
	Avatar    string    `bson:"avatar" json:"avatar"`
	Online    bool      `bson:"online" json:"online"`
	LastSeen  time.Time `bson:"last_seen" json:"last_seen"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=20"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UpdateProfileRequest struct {
	Username string `json:"username" validate:"min=3,max=20"`
	Bio      string `json:"bio" validate:"max=500"`
	Avatar   string `json:"avatar" validate:"url"`
}

// Validation methods
func (r *RegisterRequest) Validate() []string {
	var errors []string

	if r.Username == "" || len(r.Username) < 3 || len(r.Username) > 20 {
		errors = append(errors, "Username must be 3-20 characters")
	}

	if !isValidEmail(r.Email) {
		errors = append(errors, "Invalid email format")
	}

	if len(r.Password) < 6 {
		errors = append(errors, "Password must be at least 6 characters")
	}

	// Check for valid username characters
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, r.Username); !matched {
		errors = append(errors, "Username can only contain letters, numbers, and underscores")
	}

	return errors
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(strings.ToLower(email))
}
