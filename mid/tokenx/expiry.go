package tokenx

import (
	"time"

	configure "github.com/jom-io/gorig/utils/cofigure"
)

// normalizeExpireSeconds accepts the documented duration in seconds and keeps
// compatibility with callers that historically passed an absolute Unix time.
func normalizeExpireSeconds(value int64) int64 {
	now := time.Now().Unix()
	if value <= 0 {
		return int64(configure.GetInt("Jwt.TokenExpireAt", defExpire))
	}
	if value > now {
		return value - now
	}
	return value
}
