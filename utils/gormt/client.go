package gormt

import (
	"errors"
	"fmt"
	"github.com/WnJee/gorig/global/errc"
	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/logger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLog "gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
	"strings"
	"time"
)

// GetOneMysqlClient 获取一个 mysql 客户端
func GetOneMysqlClient(sqlName string) (*gorm.DB, error) {
	sqlType := "mysql"
	readDbIsOpen := configure.GetInt(sqlType + "." + sqlName + ".is_open_read_db")
	return GetSqlDriver(sqlType, sqlName, readDbIsOpen)
}

// GetSqlDriver 获取数据库驱动, 可以通过options 动态参数连接任意多个数据库
func GetSqlDriver(sqlType string, sqlName string, readDbIsOpen int, dbConf ...ConfigParams) (*gorm.DB, error) {

	var dbDialector gorm.Dialector
	if val, err := getDbDialector(sqlType, sqlName, "write", dbConf...); err != nil {
		logger.Logger.Error(errc.ErrorsDialectorDbInitFail+sqlType, zap.Error(err))
		return nil, err
	} else {
		dbDialector = val
	}
	gormDb, err := gorm.Open(dbDialector, &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		Logger:                 redefineLog(sqlName), //拦截、接管 gorm v2 自带日志
	})
	if err != nil {
		//gorm 数据库驱动初始化失败
		return nil, err
	}

	// 如果开启了读写分离，配置读数据库（resource、read、replicas）
	// 读写分离配置只
	if readDbIsOpen == 1 {
		if val, err := getDbDialector(sqlType, sqlName, "read", dbConf...); err != nil {
			logger.Logger.Error(errc.ErrorsDialectorDbInitFail+sqlType, zap.Error(err))
		} else {
			dbDialector = val
		}
		resolverConf := dbresolver.Config{
			Replicas: []gorm.Dialector{dbDialector}, //  读 操作库，查询类
			Policy:   dbresolver.RandomPolicy{},     // sources/replicas 负载均衡策略适用于
		}
		readPrefix := sqlType + "." + sqlName + ".read"
		err = gormDb.Use(dbresolver.Register(resolverConf).SetConnMaxIdleTime(time.Second * 30).
			SetConnMaxLifetime(configure.GetDuration(readPrefix+".conn_max_lifetime") * time.Second).
			SetMaxIdleConns(configure.GetInt(readPrefix + ".max_idle_conns")).
			SetMaxOpenConns(configure.GetInt(readPrefix + ".max_open_conns")))
		if err != nil {
			return nil, err
		}
	}

	// 查询没有数据，屏蔽 gorm v2 包中会爆出的错误
	// https://github.com/go-gorm/gorm/issues/3789  此 issue 所反映的问题就是我们本次解决掉的
	_ = gormDb.Callback().Query().Before("gorm:query").Register("disable_raise_record_not_found", MaskNotDataError)

	// https://github.com/go-gorm/gorm/issues/4838
	_ = gormDb.Callback().Create().Before("gorm:before_create").Register("CreateBeforeHook", CreateBeforeHook)
	// 为了完美支持gorm的一系列回调函数
	_ = gormDb.Callback().Update().Before("gorm:before_update").Register("UpdateBeforeHook", UpdateBeforeHook)

	// 为主连接设置连接池(43行返回的数据库驱动指针)
	if rawDb, err := gormDb.DB(); err != nil {
		return nil, err
	} else {
		writePrefix := sqlType + "." + sqlName + ".write"
		rawDb.SetConnMaxIdleTime(time.Second * 30)
		rawDb.SetConnMaxLifetime(configure.GetDuration(writePrefix+".conn_max_lifetime") * time.Second)
		rawDb.SetMaxIdleConns(configure.GetInt(writePrefix + ".max_idle_conns"))
		rawDb.SetMaxOpenConns(configure.GetInt(writePrefix + ".max_open_conns"))
		return gormDb, nil
	}
}

// 获取一个数据库方言(Dialector),通俗的说就是根据不同的连接参数，获取具体的一类数据库的连接指针
func getDbDialector(sqlType, sqlName, readWrite string, dbConf ...ConfigParams) (gorm.Dialector, error) {
	var dbDialector gorm.Dialector
	dsn := getDsn(sqlType, sqlName, readWrite, dbConf...)
	switch strings.ToLower(sqlType) {
	case "mysql":
		dbDialector = mysql.Open(dsn)
	//case "sqlserver", "mssql":
	//	dbDialector = sqlserver.Open(dsn)
	//case "postgres", "postgresql", "postgre":
	//	dbDialector = postgres.Open(dsn)
	default:
		return nil, errors.New(errc.ErrorsDbDriverNotExists + sqlType)
	}
	return dbDialector, nil
}

// 根据配置参数生成数据库驱动 dsn
func getDsn(sqlType, sqlName, readWrite string, dbConf ...ConfigParams) string {
	prefix := sqlType + "." + sqlName + "." + readWrite
	host := configure.GetString(prefix + ".host")
	database := configure.GetString(prefix + ".database")
	port := configure.GetInt(prefix + ".port")
	user := configure.GetString(prefix + ".user")
	pass := configure.GetString(prefix + ".pass")
	charset := configure.GetString(prefix + ".charset", "utf8mb4")

	if len(dbConf) > 0 {
		if strings.ToLower(readWrite) == "write" {
			if len(dbConf[0].Write.Host) > 0 {
				host = dbConf[0].Write.Host
			}
			if len(dbConf[0].Write.DataBase) > 0 {
				database = dbConf[0].Write.DataBase
			}
			if dbConf[0].Write.Port > 0 {
				port = dbConf[0].Write.Port
			}
			if len(dbConf[0].Write.User) > 0 {
				user = dbConf[0].Write.User
			}
			if len(dbConf[0].Write.Pass) > 0 {
				pass = dbConf[0].Write.Pass
			}
			if len(dbConf[0].Write.Charset) > 0 {
				charset = dbConf[0].Write.Charset
			}
		} else {
			if len(dbConf[0].Read.Host) > 0 {
				host = dbConf[0].Read.Host
			}
			if len(dbConf[0].Read.DataBase) > 0 {
				database = dbConf[0].Read.DataBase
			}
			if dbConf[0].Read.Port > 0 {
				port = dbConf[0].Read.Port
			}
			if len(dbConf[0].Read.User) > 0 {
				user = dbConf[0].Read.User
			}
			if len(dbConf[0].Read.Pass) > 0 {
				pass = dbConf[0].Read.Pass
			}
			if len(dbConf[0].Read.Charset) > 0 {
				charset = dbConf[0].Read.Charset
			}
		}
	}

	switch strings.ToLower(sqlType) {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local", user, pass, host, port, database, charset)
	case "sqlserver", "mssql":
		return fmt.Sprintf("server=%s;port=%d;database=%s;user id=%s;password=%s;encrypt=disable", host, port, database, user, pass)
	case "postgresql", "postgre", "postgres":
		return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable TimeZone=Asia/Shanghai", host, port, database, user, pass)
	}
	return ""
}

// 创建自定义日志模块，对 gorm 日志进行拦截、
func redefineLog(sqlType string) gormLog.Interface {
	return createCustomGormLog(sqlType,
		SetInfoStrFormat("[info] %s\n"), SetWarnStrFormat("[warn] %s\n"), SetErrStrFormat("[error] %s\n"),
		SetTraceStrFormat("[traceStr] %s [%.3fms] [rows:%v] %s\n"), SetTracWarnStrFormat("[traceWarn] %s %s [%.3fms] [rows:%v] %s\n"), SetTracErrStrFormat("[traceErr] %s %s [%.3fms] [rows:%v] %s\n"))
}
