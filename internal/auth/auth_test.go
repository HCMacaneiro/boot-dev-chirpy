package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPasswordHashing handles the Argon2id check
func TestPasswordHashing(t *testing.T) {
	password := "super_secret_password_123"

	// Test hashing works
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error when hashing, got: %v", err)
	}
	if hash == "" {
		t.Fatalf("Expected a valid hash string, got empty string")
	}

	// Test correct password validation
	match, err := CheckPasswordHash(password, hash)
	if err != nil {
		t.Fatalf("Expected no error when checking hash, got: %v", err)
	}
	if !match {
		t.Errorf("Expected password to match the generated hash")
	}

	// Test wrong password validation fails
	wrongMatch, err := CheckPasswordHash("wrong_password", hash)
	if err != nil {
		t.Fatalf("Expected no error when checking wrong password, got: %v", err)
	}
	if wrongMatch {
		t.Errorf("Expected wrong password to fail validation against hash")
	}
}

// TestJWT Lifecycle handles token generation, verification, and expiration
func TestJWTLifecycle(t *testing.T) {
	userID := uuid.New()
	secret := "my_motherfucking_ultra_secure_secret_key_12345"
	duration := 1 * time.Hour

	// 1. Test Valid JWT Generation and Validation
	tokenStr, err := MakeJWT(userID, secret, duration)
	if err != nil {
		t.Fatalf("Expected no error making JWT, got: %v", err)
	}

	parsedID, err := ValidateJWT(tokenStr, secret)
	if err != nil {
		t.Fatalf("Expected valid token to pass verification, got error: %v", err)
	}
	if parsedID != userID {
		t.Errorf("Expected parsed UUID %v to equal original UUID %v", parsedID, userID)
	}

	// 2. Test Wrong Secret Rejection
	_, err = ValidateJWT(tokenStr, "wrong_secret_key_garbage")
	if err == nil {
		t.Errorf("Expected validation to fail with a bad secret key, but it passed")
	}

	// 3. Test Expired Token Rejection
	expiredDuration := -1 * time.Hour // Forces token to be issued in the past
	expiredTokenStr, err := MakeJWT(userID, secret, expiredDuration)
	if err != nil {
		t.Fatalf("Expected no error making expired JWT, got: %v", err)
	}

	_, err = ValidateJWT(expiredTokenStr, secret)
	if err == nil {
		t.Errorf("Expected validation to fail for an expired token, but it passed")
	}
}
