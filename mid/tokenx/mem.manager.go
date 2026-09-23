package tokenx

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/WnJee/gorig/global/consts"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/WnJee/gorig/utils/sys"
	"go.uber.org/zap"
)

type tokenInfo struct {
	UserID      string
	UserType    string
	ExpiresAt   int64
	LastRefresh int64
}

// var tokens = make(map[string]*tokenInfo)
var tokenMap = sync.Map{}
var localTokensFile = "./tokens.json"
var refreshGap = int64(60 * 60)

type memoryImpl struct {
	generator TokenGenerator
}

func init() {
	sys.Info("# Token memory manager initialized")
	loadLocalTokens()

	//c := make(chan os.Signal, 1)
	//signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	//go func() {
	//	sig := <-c
	//	logger.Info(nil, fmt.Sprintf("received signal:%v", sig))
	//	saveLocalTokens()
	//	os.Exit(0)
	//}()

	// 定时清理过期token
	go func() {
		for {
			time.Sleep(time.Second * 60)
			tokenMap.Range(func(key, value interface{}) bool {
				userInfoGet := value.(*tokenInfo)
				if userInfoGet != nil && userInfoGet.ExpiresAt < time.Now().Unix() {
					tokenMap.Delete(key)
				}
				return true
			})
			go saveLocalTokens()
		}
	}()
}

func loadLocalTokens() {
	if _, err := os.Stat(localTokensFile); os.IsNotExist(err) {
		return
	}

	file, err := os.Open(localTokensFile)
	if err != nil {
		logger.Error(nil, fmt.Sprintf("Open tokens file error: %v", err))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		logger.Error(nil, fmt.Sprintf("Read tokens file error: %v", err))
		return
	}

	if len(data) == 0 {
		//logger.Info(nil, "Tokens file is empty")
		return
	}

	mapData := make(map[string]*tokenInfo)
	err = json.Unmarshal(data, &mapData)
	if err != nil {
		logger.Error(nil, fmt.Sprintf("Parse tokens JSON error: %v", err))
		// 可选：删除文件
		err = os.Remove(localTokensFile)
		if err != nil {
			logger.Error(nil, fmt.Sprintf("Delete invalid tokens file error: %v", err))
		} else {
			logger.Info(nil, "Deleted invalid tokens file")
		}
		return
	}

	for k, v := range mapData {
		tokenMap.Store(k, v)
	}
	//logger.Info(nil, "Loaded tokens from file", zap.Int("count", len(mapData)))
}

var saveLock = make(chan struct{}, 1)

func tokenLen() int {
	count := 0
	tokenMap.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func saveLocalTokens() {
	saveLock <- struct{}{}
	defer func() {
		<-saveLock
	}()

	//if tokenLen() == 0 {
	//	return
	//}

	mapData := make(map[string]*tokenInfo)
	count := 0
	tokenMap.Range(func(key, value interface{}) bool {
		mapData[key.(string)] = value.(*tokenInfo)
		count++
		return true
	})

	//if count == 0 {
	//	return
	//}

	data, err := json.Marshal(mapData)
	if err != nil {
		logger.Error(nil, fmt.Sprintf("Convert to JSON error:%v", err))
		return
	}

	tmpFile := localTokensFile + ".tmp"
	err = os.WriteFile(tmpFile, data, 0600)
	if err == nil {
		err = os.Rename(tmpFile, localTokensFile)
	}
	if err != nil {
		logger.Error(nil, fmt.Sprintf("Write file error:%v", err))
		_ = os.Remove(tmpFile)
		return
	}
	_ = os.Chmod(localTokensFile, 0600)
}

var tokenLock = sync.Map{}

func getTokenLock(token string) *sync.Mutex {
	lock, exist := tokenLock.Load(token)
	if !exist {
		lock = &sync.Mutex{}
		tokenLock.Store(token, lock)
	}
	return lock.(*sync.Mutex)
}

// GetUserID
func (u *memoryImpl) GetUserID(token string) (string, bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(nil, fmt.Sprintf("GetUserID panic:%v", err))
		}
	}()

	lock := getTokenLock(token)
	lock.Lock()
	defer lock.Unlock()

	if tokenLen() == 0 {
		loadLocalTokens()
	}

	value, exists := tokenMap.Load(token)
	if !exists {
		return "", false
	}

	userInfo := value.(*tokenInfo)
	if userInfo == nil || userInfo.ExpiresAt < time.Now().Unix() {
		u.Destroy(token)
		go saveLocalTokens()
		return "", false
	}

	now := time.Now().Unix()

	if userInfo.LastRefresh == 0 {
		userInfo.LastRefresh = userInfo.ExpiresAt - int64(configure.GetInt("Jwt.TokenExpireAt", defExpire))
	}

	if now-userInfo.LastRefresh >= refreshGap {
		userInfo.ExpiresAt = now + int64(configure.GetInt("Jwt.TokenExpireAt", defExpire))
		userInfo.LastRefresh = now
		tokenMap.Store(token, userInfo)
		go saveLocalTokens()
	}

	return userInfo.UserID, true
}

// 根据userInfo获取userType 规则为: userInfo的value拼接 用于token变动后及时失效
func getUserType(userInfo map[string]interface{}) string {
	data, err := json.Marshal(userInfo)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (u *memoryImpl) GenerateAndRecord(ctx context.Context, userId string, userInfo map[string]interface{}, expireAt int64) (token string, err *errors.Error) {
	logger.Info(ctx, "GenerateAndRecord", zap.String("user_id", userId))
	expireAt = normalizeExpireSeconds(expireAt)
	tokenMap.Range(func(key, value interface{}) bool {
		userInfoGet := value.(*tokenInfo)
		if userInfoGet != nil && userInfoGet.ExpiresAt <= time.Now().Unix() {
			tokenMap.Delete(key)
			return true
		}
		if userInfoGet != nil && userInfoGet.UserID == userId && userInfoGet.UserType == getUserType(userInfo) {
			token = key.(string)
			return false
		}
		return true
	})
	if token != "" {
		return
	}
	if token, err = u.generator.Generate(userId, userInfo, expireAt); err == nil {
		u.Clean(userId)
		u.Record(token, userInfo)
		go saveLocalTokens()
	}
	return
}

func (u *memoryImpl) Record(userToken string, userInfo map[string]interface{}) bool {
	if customClaims, err := u.generator.Parse(userToken); err == nil {
		userId := customClaims.UserId
		//expiresAt := customClaims.ExpiresAt
		expireAt := customClaims.ExpiresAtUnix()
		tokenMap.Store(userToken, &tokenInfo{UserID: userId, UserType: getUserType(userInfo), ExpiresAt: expireAt})
		//logger.Info(nil, fmt.Sprintf("Record userToken:%s userId:%s userInfo:%v expireAt:%d", userToken, userId, userInfo, expireAt))
		return true
	} else {
		return false
	}
}

// IsMeetRefresh 检查token是否满足刷新条件
func (u *memoryImpl) IsMeetRefresh(token string) bool {
	// token基本信息是否有效：1.过期时间在允许的过期范围内;2.基本格式正确
	_, code := u.IsNotExpired(token, int64(configure.GetInt("Jwt.TokenRefreshAllowSec")))
	return code == consts.JwtTokenOK
}

func (u *memoryImpl) Refresh(oldToken string, newToken string) (res bool) {
	oldClaims, oldErr := u.generator.Parse(oldToken)
	newClaims, newErr := u.generator.Parse(newToken)
	if oldErr != nil || newErr != nil || oldClaims.UserId != newClaims.UserId {
		return false
	}
	tokenMap.Delete(oldToken)
	tokenMap.Store(newToken, &tokenInfo{
		UserID:    newClaims.UserId,
		UserType:  getUserType(newClaims.UserInfo),
		ExpiresAt: newClaims.ExpiresAtUnix(),
	})
	go saveLocalTokens()
	return true
}

// IsNotExpired
func (u *memoryImpl) IsNotExpired(token string, expireAtSec int64) (*CustomClaims, int) {
	if customClaims, err := u.generator.Parse(token); err == nil {
		if time.Now().Unix()-(customClaims.ExpiresAtUnix()+expireAtSec) < 0 {
			return customClaims, consts.JwtTokenOK
		} else {
			return customClaims, consts.JwtTokenExpired
		}
	} else {
		return nil, consts.JwtTokenInvalid
	}
}

// IsEffective 判断token是否有效（未过期+数据库用户信息正常）
func (u *memoryImpl) IsEffective(token string) bool {
	_, code := u.IsNotExpired(token, 0)
	if consts.JwtTokenOK != code {
		return false
	}
	value, ok := tokenMap.Load(token)
	if !ok {
		return false
	}
	info, ok := value.(*tokenInfo)
	return ok && info != nil && info.UserID != "" && info.ExpiresAt >= time.Now().Unix()
}

func (u *memoryImpl) Destroy(token string) {
	logger.Info(nil, "Destroy token")
	tokenMap.Delete(token)
}

func (u *memoryImpl) CleanAll() {
	tokenMap = sync.Map{}
}

func (u *memoryImpl) Clean(userId string) {
	tokenMap.Range(func(key, value interface{}) bool {
		userInfoGet := value.(*tokenInfo)
		if userInfoGet != nil && userInfoGet.UserID == userId {
			tokenMap.Delete(key)
		}
		return true
	})
}
