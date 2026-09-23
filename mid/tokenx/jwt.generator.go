package tokenx

import (
	stderrors "errors"
	"github.com/WnJee/gorig/global/errc"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type jwtGenerator struct {
	SigningKey []byte
}

func (j *jwtGenerator) Generate(userId string, userInfo map[string]interface{}, expireAt int64) (tokens string, err *errors.Error) {
	if expireAt <= 0 {
		expireAt = defExpire
	}
	now := time.Now()
	claims := CustomClaims{
		UserId:   userId,
		UserInfo: userInfo,
		RegisteredClaims: jwt.RegisteredClaims{
			NotBefore: jwt.NewNumericDate(now.Add(-10 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireAt) * time.Second)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedString, signErr := token.SignedString(j.SigningKey)
	if signErr != nil {
		return "", errors.Sys("jwt sign string error", signErr)
	}
	return signedString, nil
}

func (j *jwtGenerator) Parse(token string) (*CustomClaims, *errors.Error) {
	if customClaims, err := j.ParseToken(token); err == nil {
		return customClaims, nil
	} else {
		return &CustomClaims{}, err
	}
}

func (j *jwtGenerator) ParseToken(tokenString string) (*CustomClaims, *errors.Error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if token == nil {
		return nil, errors.Verify(errc.ErrorsTokenInvalid)
	}
	if err != nil {
		switch {
		case stderrors.Is(err, jwt.ErrTokenMalformed):
			return nil, errors.Verify(errc.ErrorsTokenMalFormed)
		case stderrors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, errors.Verify(errc.ErrorsTokenNotActiveYet)
		case isOnlyExpired(err):
			// An expired token is intentionally returned so token refresh can
			// inspect its claims, but only when expiration is the sole validation
			// failure. In particular, never turn an expired token with a bad
			// signature into a valid token.
			token.Valid = true
		default:
			return nil, errors.Verify(errc.ErrorsTokenInvalid)
		}
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	} else {
		return nil, errors.Verify(errc.ErrorsTokenInvalid)
	}
}

// isOnlyExpired reports whether the parse failure is exactly token expiry,
// with no malformed, signature, or validity problem attached.
func isOnlyExpired(err error) bool {
	if err == nil {
		return false
	}
	for _, other := range []error{
		jwt.ErrTokenMalformed,
		jwt.ErrTokenUnverifiable,
		jwt.ErrTokenSignatureInvalid,
		jwt.ErrTokenNotValidYet,
		jwt.ErrTokenUsedBeforeIssued,
	} {
		if stderrors.Is(err, other) {
			return false
		}
	}
	return stderrors.Is(err, jwt.ErrTokenExpired)
}
