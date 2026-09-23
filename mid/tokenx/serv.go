package tokenx

import (
	"context"
	"crypto/rand"
	"github.com/WnJee/gorig/global/variable"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/logger"
	"sync"
)

type GeneratorType int

const (
	Jwt = iota
)

type ManagerType int

const (
	Memory = iota
	Redis
)

const defExpire = 3600 * 24 * 7

var (
	processSigningKey []byte
	processKeyOnce    sync.Once
)

func fallbackSigningKey() []byte {
	processKeyOnce.Do(func() {
		processSigningKey = make([]byte, 32)
		if _, err := rand.Read(processSigningKey); err != nil {
			panic("tokenx: unable to initialize secure signing key: " + err.Error())
		}
	})
	return processSigningKey
}

type TokenGenerator interface {
	Generate(userId string, userInfo map[string]interface{}, expireAt int64) (tokens string, err *errors.Error)
	Parse(token string) (*CustomClaims, *errors.Error)
}

type TokenManager interface {
	Record(userToken string, userInfo map[string]interface{}) bool
	GenerateAndRecord(ctx context.Context, userId string, userInfo map[string]interface{}, expireAt int64) (tokens string, err *errors.Error)
	IsNotExpired(token string, allowSec int64) (*CustomClaims, int)
	IsMeetRefresh(token string) bool
	Refresh(oldToken, newToken string) bool
	IsEffective(token string) bool
	Destroy(token string)
	GetUserID(token string) (userID string, exist bool)
	CleanAll()
	Clean(userId string)
}

type TokenService struct {
	Generator TokenGenerator
	Manager   TokenManager
}

func GetDef() *TokenService {
	return Get(Jwt, Memory)
}

func Get(generatorType GeneratorType, managerType ManagerType) *TokenService {
	generator := getGenerator(generatorType)
	return &TokenService{
		Generator: getGenerator(generatorType),
		Manager:   getManager(managerType, generator),
	}
}

func getGenerator(generatorType GeneratorType) TokenGenerator {
	sign := variable.JwtKey
	if sign == "" {
		sign = variable.SysName
	}
	switch generatorType {
	case Jwt:
		key := []byte(sign)
		if len(key) == 0 {
			logger.Warn(nil, "tokenx: jwt.key/sys.name not configured; using a random per-process signing key. "+
				"Tokens will be invalidated on every restart and are not valid across instances. "+
				"Set jwt.key in the configuration file or GORIG_JWT_KEY environment variable.")
			key = fallbackSigningKey()
		}
		return &jwtGenerator{
			SigningKey: key,
		}
	}
	return nil
}

func getManager(managerType ManagerType, generator TokenGenerator) TokenManager {
	switch managerType {
	case Memory:
		return &memoryImpl{
			generator: generator,
		}
	case Redis:
		return newRedisImpl(generator)
	default:
		return &memoryImpl{
			generator: generator,
		}
	}
}
