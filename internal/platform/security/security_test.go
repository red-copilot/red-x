package security

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("password should verify")
	}
	if VerifyPassword("wrong password", hash) {
		t.Fatal("wrong password must not verify")
	}
}

func TestAccessToken(t *testing.T) {
	token, err := NewAccessToken("secret", "user-1", "admin", "admin", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken("secret", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" || claims.SessionID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := ParseAccessToken("different", token); err == nil {
		t.Fatal("token signed with another secret must fail")
	}
}

func TestAPIKeyHash(t *testing.T) {
	a := APIKeyHash("raw", "pepper")
	b := APIKeyHash("raw", "pepper")
	c := APIKeyHash("raw", "other")
	if a != b || a == c {
		t.Fatal("api key hash must be deterministic and pepper-dependent")
	}
}
