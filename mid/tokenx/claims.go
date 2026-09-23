package tokenx

import "github.com/golang-jwt/jwt/v5"

type CustomClaims struct {
	UserId   string
	UserInfo map[string]interface{}
	jwt.RegisteredClaims
}

// ExpiresAtUnix returns the token expiry as Unix seconds, 0 when absent.
// It keeps the historical int64 contract used by the memory and Redis managers.
func (c *CustomClaims) ExpiresAtUnix() int64 {
	if c.ExpiresAt == nil {
		return 0
	}
	return c.ExpiresAt.Unix()
}
