package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/darren-you/auth_service/template_server/pkg/session"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	Server ServerConfig `json:"server"`
	MySQL  MySQLConfig  `json:"mysql"`
	Redis  RedisConfig  `json:"redis"`
	Log    LogConfig    `json:"log"`
	JWT    JWTConfig    `json:"jwt"`
	Auth   AuthConfig   `json:"auth"`
}

type ServerConfig struct {
	Name             string               `json:"name,default=auth_service"`
	Mode             string               `json:"mode,default=pro,options=dev|test|rt|pre|pro"`
	Host             string               `json:"host,default=0.0.0.0"`
	Port             int                  `json:"port,default=8080"`
	ReadTimeout      int                  `json:"read_timeout,default=30"`
	WriteTimeout     int                  `json:"write_timeout,default=30"`
	RequestTimeoutMS int64                `json:"timeout,optional"`
	MaxConns         int                  `json:"max_conns,default=10000"`
	MaxBytes         int64                `json:"max_bytes,default=1048576"`
	Middlewares      rest.MiddlewaresConf `json:"middlewares,optional"`
	TraceIgnorePaths []string             `json:"trace_ignore_paths,optional"`
	AllowOrigins     []string             `json:"allow_origins,optional"`
	AllowMethods     []string             `json:"allow_methods,optional"`
	AllowHeaders     []string             `json:"allow_headers,optional"`
}

type MySQLConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password,optional"`
	Database        string `json:"database"`
	MaxOpenConns    int    `json:"max_open_conns,optional"`
	MaxIdleConns    int    `json:"max_idle_conns,optional"`
	ConnMaxLifetime int    `json:"conn_max_lifetime,optional"`
	ConnMaxIdleTime int    `json:"conn_max_idle_time,optional"`
}

type RedisConfig struct {
	Addr           string `json:"addr"`
	Password       string `json:"password,optional"`
	DB             int    `json:"db,optional"`
	PoolSize       int    `json:"pool_size,optional"`
	MinIdleConns   int    `json:"min_idle_conns,optional"`
	DialTimeoutMS  int    `json:"dial_timeout_ms,optional"`
	ReadTimeoutMS  int    `json:"read_timeout_ms,optional"`
	WriteTimeoutMS int    `json:"write_timeout_ms,optional"`
	PoolTimeoutMS  int    `json:"pool_timeout_ms,optional"`
}

type LogConfig struct {
	Level    string `json:"level,default=info,options=debug|info|error|severe"`
	File     string `json:"file,optional"`
	Encoding string `json:"encoding,default=json,options=plain|json"`
}

type JWTConfig struct {
	Secret             string `json:"secret"`
	Issuer             string `json:"issuer,default=auth_service"`
	AccessExpiry       int    `json:"access_expiry,default=3600"`
	RefreshExpiry      int    `json:"refresh_expiry,default=604800"`
	AccessTokenType    string `json:"access_token_type,default=access"`
	RefreshTokenType   string `json:"refresh_token_type,default=refresh"`
	ExpiringSoonWindow int    `json:"expiring_soon_window,default=900"`
}

type AuthConfig struct {
	StateTTLSecond        int            `json:"state_ttl_second,default=300"`
	PhoneCaptchaTTLSecond int            `json:"phone_captcha_ttl_second,default=300"`
	GuestDeviceTTLSecond  int            `json:"guest_device_ttl_second,default=600"`
	Tenants               []TenantConfig `json:"tenants,optional"`
}

type TenantConfig struct {
	Key           string           `json:"key"`
	Name          string           `json:"name"`
	Enabled       bool             `json:"enabled"`
	BridgeBaseURL string           `json:"bridge_base_url,optional"`
	BridgeAuthKey string           `json:"bridge_auth_key,optional"`
	Providers     []ProviderConfig `json:"providers,optional"`
}

type ProviderConfig struct {
	Provider       string `json:"provider"`
	ClientType     string `json:"client_type"`
	Enabled        bool   `json:"enabled"`
	AppID          string `json:"app_id,optional"`
	AppSecret      string `json:"app_secret,optional"`
	RedirectURI    string `json:"redirect_uri,optional"`
	Scope          string `json:"scope,optional"`
	TeamID         string `json:"team_id,optional"`
	ClientID       string `json:"client_id,optional"`
	KeyID          string `json:"key_id,optional"`
	SigningKey     string `json:"signing_key,optional"`
	TestPhone      string `json:"test_phone,optional"`
	TestCaptcha    string `json:"test_captcha,optional"`
	TestCaptchaKey string `json:"test_captcha_key,optional"`
	ExtraJSON      string `json:"extra_json,optional"`
}

func (c *Config) Validate() error {
	normalizeConfig(c)

	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be greater than 0")
	}
	if strings.TrimSpace(c.MySQL.Host) == "" || c.MySQL.Port <= 0 || strings.TrimSpace(c.MySQL.User) == "" || strings.TrimSpace(c.MySQL.Database) == "" {
		return fmt.Errorf("mysql config is incomplete")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return fmt.Errorf("redis.addr is required")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("jwt.secret is required")
	}

	return nil
}

func (c ServerConfig) RestConf(logConf logx.LogConf) rest.RestConf {
	timeout := c.RequestTimeoutMS
	if timeout <= 0 {
		timeout = int64(maxInt(c.ReadTimeout, c.WriteTimeout)) * int64(time.Second/time.Millisecond)
	}

	return rest.RestConf{
		ServiceConf: service.ServiceConf{
			Name: c.Name,
			Log:  logConf,
			Mode: c.Mode,
		},
		Host:             c.Host,
		Port:             c.Port,
		Timeout:          timeout,
		MaxConns:         c.MaxConns,
		MaxBytes:         c.MaxBytes,
		Middlewares:      c.Middlewares,
		TraceIgnorePaths: c.TraceIgnorePaths,
	}
}

func (c MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
	)
}

func (c LogConfig) LogConf(serviceName string) logx.LogConf {
	path := resolveLogDir(c.File)
	mode := "console"
	if path != "" {
		mode = "file"
	}

	return logx.LogConf{
		ServiceName: serviceName,
		Mode:        mode,
		Encoding:    fallbackString(c.Encoding, "json"),
		Path:        path,
		Level:       fallbackString(c.Level, "info"),
		Stat:        true,
	}
}

func (c JWTConfig) SessionConfig() session.Config {
	return session.Config{
		SecretKey:          c.Secret,
		Issuer:             c.Issuer,
		AccessExpiry:       time.Duration(c.AccessExpiry) * time.Second,
		RefreshExpiry:      time.Duration(c.RefreshExpiry) * time.Second,
		AccessTokenType:    c.AccessTokenType,
		RefreshTokenType:   c.RefreshTokenType,
		ExpiringSoonWindow: time.Duration(c.ExpiringSoonWindow) * time.Second,
	}
}

func normalizeConfig(cfg *Config) {
	cfg.Server.Name = fallbackString(cfg.Server.Name, "auth_service")
	cfg.Server.Mode = fallbackString(cfg.Server.Mode, service.ProMode)
	cfg.Server.Host = fallbackString(cfg.Server.Host, "0.0.0.0")
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout <= 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout <= 0 {
		cfg.Server.WriteTimeout = 30
	}
	if cfg.Server.MaxConns <= 0 {
		cfg.Server.MaxConns = 10000
	}
	if cfg.Server.MaxBytes <= 0 {
		cfg.Server.MaxBytes = 1048576
	}
	if len(cfg.Server.AllowOrigins) == 0 {
		cfg.Server.AllowOrigins = []string{"*"}
	}
	if len(cfg.Server.AllowMethods) == 0 {
		cfg.Server.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(cfg.Server.AllowHeaders) == 0 {
		cfg.Server.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}

	cfg.MySQL.Host = fallbackString(cfg.MySQL.Host, "127.0.0.1")
	if cfg.MySQL.Port <= 0 {
		cfg.MySQL.Port = 3306
	}
	cfg.MySQL.User = fallbackString(cfg.MySQL.User, "root")
	cfg.MySQL.Database = fallbackString(cfg.MySQL.Database, "auth_service")
	if cfg.MySQL.MaxOpenConns <= 0 {
		cfg.MySQL.MaxOpenConns = 100
	}
	if cfg.MySQL.MaxIdleConns <= 0 {
		cfg.MySQL.MaxIdleConns = 10
	}
	if cfg.MySQL.ConnMaxLifetime <= 0 {
		cfg.MySQL.ConnMaxLifetime = 3600
	}
	if cfg.MySQL.ConnMaxIdleTime <= 0 {
		cfg.MySQL.ConnMaxIdleTime = cfg.MySQL.ConnMaxLifetime
	}

	cfg.Redis.Addr = fallbackString(cfg.Redis.Addr, "127.0.0.1:6379")
	if cfg.Redis.PoolSize <= 0 {
		cfg.Redis.PoolSize = 10
	}
	if cfg.Redis.MinIdleConns <= 0 {
		cfg.Redis.MinIdleConns = maxInt(cfg.Redis.PoolSize/2, 1)
	}
	if cfg.Redis.DialTimeoutMS <= 0 {
		cfg.Redis.DialTimeoutMS = 5000
	}
	if cfg.Redis.ReadTimeoutMS <= 0 {
		cfg.Redis.ReadTimeoutMS = 3000
	}
	if cfg.Redis.WriteTimeoutMS <= 0 {
		cfg.Redis.WriteTimeoutMS = 3000
	}
	if cfg.Redis.PoolTimeoutMS <= 0 {
		cfg.Redis.PoolTimeoutMS = 4000
	}

	cfg.Log.Level = fallbackString(cfg.Log.Level, "info")
	cfg.Log.Encoding = fallbackString(cfg.Log.Encoding, "json")
	cfg.Log.File = fallbackString(cfg.Log.File, "logs/app.log")

	cfg.JWT.Secret = strings.TrimSpace(cfg.JWT.Secret)
	cfg.JWT.Issuer = fallbackString(cfg.JWT.Issuer, "auth_service")
	if cfg.JWT.AccessExpiry <= 0 {
		cfg.JWT.AccessExpiry = 3600
	}
	if cfg.JWT.RefreshExpiry <= 0 {
		cfg.JWT.RefreshExpiry = 604800
	}
	cfg.JWT.AccessTokenType = fallbackString(cfg.JWT.AccessTokenType, "access")
	cfg.JWT.RefreshTokenType = fallbackString(cfg.JWT.RefreshTokenType, "refresh")
	if cfg.JWT.ExpiringSoonWindow <= 0 {
		cfg.JWT.ExpiringSoonWindow = 900
	}

	if cfg.Auth.StateTTLSecond <= 0 {
		cfg.Auth.StateTTLSecond = 300
	}
	if cfg.Auth.PhoneCaptchaTTLSecond <= 0 {
		cfg.Auth.PhoneCaptchaTTLSecond = 300
	}
	if cfg.Auth.GuestDeviceTTLSecond <= 0 {
		cfg.Auth.GuestDeviceTTLSecond = 600
	}

	for tenantIndex := range cfg.Auth.Tenants {
		tenant := &cfg.Auth.Tenants[tenantIndex]
		tenant.Key = normalizeKey(tenant.Key)
		tenant.Name = strings.TrimSpace(tenant.Name)
		tenant.BridgeBaseURL = strings.TrimSpace(tenant.BridgeBaseURL)
		tenant.BridgeAuthKey = strings.TrimSpace(tenant.BridgeAuthKey)
		for providerIndex := range tenant.Providers {
			provider := &tenant.Providers[providerIndex]
			provider.Provider = normalizeKey(provider.Provider)
			provider.ClientType = normalizeKey(provider.ClientType)
			provider.AppID = strings.TrimSpace(provider.AppID)
			provider.AppSecret = strings.TrimSpace(provider.AppSecret)
			provider.RedirectURI = strings.TrimSpace(provider.RedirectURI)
			provider.Scope = strings.TrimSpace(provider.Scope)
			provider.TeamID = strings.TrimSpace(provider.TeamID)
			provider.ClientID = strings.TrimSpace(provider.ClientID)
			provider.KeyID = strings.TrimSpace(provider.KeyID)
			provider.SigningKey = strings.TrimSpace(provider.SigningKey)
			provider.TestPhone = strings.TrimSpace(provider.TestPhone)
			provider.TestCaptcha = strings.TrimSpace(provider.TestCaptcha)
			provider.TestCaptchaKey = strings.TrimSpace(provider.TestCaptchaKey)
			provider.ExtraJSON = strings.TrimSpace(provider.ExtraJSON)
		}
	}
}

func fallbackString(value string, defaultValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func resolveLogDir(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "logs"
	}

	cleaned := filepath.Clean(trimmed)
	if ext := filepath.Ext(cleaned); ext != "" {
		dir := filepath.Dir(cleaned)
		if dir == "." {
			return "logs"
		}
		return dir
	}

	return cleaned
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
