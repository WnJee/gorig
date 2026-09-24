package configure

import (
	"errors"
	gerrors "github.com/WnJee/gorig/utils/errors"
	"github.com/WnJee/gorig/utils/strs"
	"github.com/spf13/viper"
	"log"
	"strings"
	"sync"
	"time"
)

func GetSub(key string) map[string]interface{} {
	return viper.GetStringMap(key)
}

func GetString(key string, def ...string) string {
	ok := exists(key)
	if ok {
		return viper.GetString(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return strs.EMPTY
}

func MustGetString(key string) (string, *gerrors.Error) {
	val := GetString(key)
	if len(val) == 0 {
		return val, gerrors.Sys("Miss configure: " + key + "! " +
			"This parameter should be set in an environment variable, startup parameter, or configuration file.")
	}
	return val, nil
}

func GetBool(key string, def ...bool) bool {
	ok := exists(key)
	if ok {
		return viper.GetBool(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return false
}

func GetInt(key string, def ...int) int {
	ok := exists(key)
	if ok {
		return viper.GetInt(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func GetUint64(key string, def ...uint64) uint64 {
	ok := exists(key)
	if ok {
		return viper.GetUint64(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func GetDuration(key string, def ...time.Duration) time.Duration {
	ok := exists(key)
	if ok {
		return viper.GetDuration(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func GetStringSlice(key string, def ...[]string) []string {
	ok := exists(key)
	if ok {
		return viper.GetStringSlice(key)
	}
	if len(def) > 0 {
		return def[0]
	}
	return nil
}

// var gConfigs = make(map[string]any)
// 改为sync.Map
var gConfigs = sync.Map{}

var (
	configName  string
	configPaths = []string{"./_bin/", "./", "../_bin/", "../", "../../_bin/", "../../"}
)

func register(key string, val any) {
	//gConfigs[key] = val
	gConfigs.Store(key, val)
}

// Dump Used to output all used configurations
func Dump(call func(key string, val any)) {
	//for k, v := range gConfigs {
	//	call(k, v)
	//}
	gConfigs.Range(func(key, value any) bool {
		call(key.(string), value)
		return true
	})
}

func exists(key string) bool {
	val := viper.Get(key)
	if val != nil {
		register(key, val)
	}
	return val != nil
}

func init() {
	viper.SetEnvPrefix("gorig")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}
	configName = GetString("sys.mode", "local")
	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			log.Fatalf("configuration file is required (name=%q, search paths=%s): %v", configName, strings.Join(configPaths, ", "), err)
		}
		log.Fatalf("failed to read configuration file (name=%q): %v", configName, err)
	}
}
