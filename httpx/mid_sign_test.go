package httpx

import (
	"testing"
)

func TestFilterUserInfoDoesNotAllowMissingUserInfo(t *testing.T) {
	if filterUserInfo(nil, map[string]interface{}{"role": "admin"}) {
		t.Fatal("missing user info must not satisfy a non-empty filter")
	}
}

func TestFilterUserInfoMatchesCommaSeparatedRolesExactly(t *testing.T) {
	userInfo := map[string]interface{}{"roles": "user,admin"}
	if !filterUserInfo(userInfo, map[string]interface{}{"roles": "admin"}) {
		t.Fatal("expected exact role match")
	}
	if filterUserInfo(userInfo, map[string]interface{}{"roles": "min"}) {
		t.Fatal("substring role match must not be accepted")
	}
}
