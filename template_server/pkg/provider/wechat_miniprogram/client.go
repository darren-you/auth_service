package wechatminiprogram

import (
	"context"

	wechatshared "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_shared"
)

type Config struct {
	AppID                string
	AppSecret            string
	APIBaseURL           string
	RequestTimeoutSecond int
}

type Client struct {
	runtime *wechatshared.Runtime
}

type MiniProgramSession = wechatshared.MiniProgramSession
type MiniProgramPhoneInfo = wechatshared.MiniProgramPhoneInfo
type APIError = wechatshared.APIError

func NewClient(cfg Config) *Client {
	return &Client{
		runtime: wechatshared.NewRuntime(wechatshared.BaseConfig{
			AppID:                cfg.AppID,
			AppSecret:            cfg.AppSecret,
			APIBaseURL:           cfg.APIBaseURL,
			RequestTimeoutSecond: cfg.RequestTimeoutSecond,
		}),
	}
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*MiniProgramSession, error) {
	return c.runtime.ExchangeMiniProgramCode(ctx, code)
}

func (c *Client) GetPhoneNumberByCode(ctx context.Context, code string) (*MiniProgramPhoneInfo, error) {
	return c.runtime.GetMiniProgramPhoneNumber(ctx, code)
}
