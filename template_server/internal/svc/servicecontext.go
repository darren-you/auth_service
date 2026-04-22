package svc

import (
	"database/sql"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/store"
	"github.com/darren-you/auth_service/template_server/pkg/session"
	goredis "github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config        config.Config
	DB            *sql.DB
	Redis         *goredis.Client
	AuthRepo      model.AuthRepository
	KVStore       store.KVStore
	SessionConfig session.Config
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	sqlx.DisableLog()

	sqlDB, err := sql.Open("mysql", c.MySQL.DSN())
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(c.MySQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(c.MySQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(c.MySQL.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(c.MySQL.ConnMaxIdleTime) * time.Second)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:         c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           c.Redis.DB,
		PoolSize:     c.Redis.PoolSize,
		MinIdleConns: c.Redis.MinIdleConns,
		DialTimeout:  time.Duration(c.Redis.DialTimeoutMS) * time.Millisecond,
		ReadTimeout:  time.Duration(c.Redis.ReadTimeoutMS) * time.Millisecond,
		WriteTimeout: time.Duration(c.Redis.WriteTimeoutMS) * time.Millisecond,
		PoolTimeout:  time.Duration(c.Redis.PoolTimeoutMS) * time.Millisecond,
	})
	if _, err := redisClient.Ping(redisClient.Context()).Result(); err != nil {
		_ = sqlDB.Close()
		_ = redisClient.Close()
		return nil, err
	}

	sqlConn := sqlx.NewSqlConnFromDB(sqlDB)

	return &ServiceContext{
		Config:        c,
		DB:            sqlDB,
		Redis:         redisClient,
		AuthRepo:      model.NewAuthRepository(sqlConn),
		KVStore:       store.NewKVStore(redisClient),
		SessionConfig: c.JWT.SessionConfig(),
	}, nil
}

func (s *ServiceContext) Close() {
	if s.DB != nil {
		_ = s.DB.Close()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
}
