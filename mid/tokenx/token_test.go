package tokenx

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"testing"
	"time"
)

func TestMemoryTokenExpiryAndEffective(t *testing.T) {
	service := GetDef()
	service.Manager.CleanAll()
	token, err := service.Manager.GenerateAndRecord(context.Background(), "user", map[string]interface{}{"role": "admin"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !service.Manager.IsEffective(token) {
		t.Fatal("new token should be effective")
	}
	time.Sleep(3 * time.Second)
	if service.Manager.IsEffective(token) {
		t.Fatal("expired token should not be effective")
	}
}

func TestParseTokenRejectsExpiredTokenWithInvalidSignature(t *testing.T) {
	generator := &jwtGenerator{SigningKey: []byte("test-signing-key")}
	claims := CustomClaims{RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(generator.SigningKey)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[2] = "invalid"
	invalid := strings.Join(parts, ".")
	if _, err := generator.ParseToken(invalid); err == nil {
		t.Fatal("expired token with an invalid signature must be rejected")
	}
}
