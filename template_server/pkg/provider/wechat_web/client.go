package wechatweb

import (
	"context"

	wechatshared "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_shared"
)

type Config struct {
	AppID                string
	AppSecret            string
	APIBaseURL           string
	ConnectBaseURL       string
	WebRedirectURI       string
	LoginScope           string
	RequestTimeoutSecond int
}

type Client struct {
	runtime        *wechatshared.Runtime
	connectBaseURL string
	webRedirectURI string
	loginScope     string
}

type OAuthToken = wechatshared.OAuthToken
type UserInfo = wechatshared.UserInfo
type AuthCheckResponse = wechatshared.AuthCheckResponse
type APIError = wechatshared.APIError

func NewClient(cfg Config) *Client {
	return &Client{
		runtime: wechatshared.NewRuntime(wechatshared.BaseConfig{
			AppID:                cfg.AppID,
			AppSecret:            cfg.AppSecret,
			APIBaseURL:           cfg.APIBaseURL,
			RequestTimeoutSecond: cfg.RequestTimeoutSecond,
		}),
		connectBaseURL: cfg.ConnectBaseURL,
		webRedirectURI: cfg.WebRedirectURI,
		loginScope:     cfg.LoginScope,
	}
}

func (c *Client) BuildLoginURL(state string) (string, error) {
	return c.runtime.BuildOAuthURL(c.connectBaseURL, c.webRedirectURI, c.loginScope, state)
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	return c.runtime.ExchangeOAuthCode(ctx, code)
}

func (c *Client) EnsureAccessTokenValid(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	return c.runtime.EnsureAccessTokenValid(ctx, token)
}

func (c *Client) FetchUserInfo(ctx context.Context, accessToken string, openID string) (*UserInfo, error) {
	return c.runtime.FetchUserInfo(ctx, accessToken, openID)
}

func IsRetryableTokenError(err error) bool {
	return wechatshared.IsRetryableTokenError(err)
}
