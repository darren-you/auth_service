package wechat

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/observability"
)

const (
	defaultAPIBaseURL     = "https://api.weixin.qq.com"
	defaultConnectBaseURL = "https://open.weixin.qq.com/connect/qrconnect"
	defaultLoginScope     = "snsapi_login"
)

var retryableTokenErrorCodes = map[int]struct{}{
	40001: {},
	40014: {},
	42001: {},
}

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
	config     Config
	httpClient *http.Client
}

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
	ExpiresIn    int    `json:"expires_in"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

type MiniProgramSession struct {
	SessionKey string `json:"session_key"`
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type UserInfo struct {
	OpenID     string `json:"openid"`
	Nickname   string `json:"nickname"`
	Sex        int    `json:"sex"`
	Province   string `json:"province"`
	City       string `json:"city"`
	Country    string `json:"country"`
	HeadImgURL string `json:"headimgurl"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type AuthCheckResponse struct {
	OpenID  string `json:"openid"`
	Scope   string `json:"scope"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("wechat api error: %d", e.Code)
	}
	return fmt.Sprintf("wechat api error: %d %s", e.Code, e.Message)
}

func NewClient(cfg Config) *Client {
	timeout := cfg.RequestTimeoutSecond
	if timeout <= 0 {
		timeout = 5
	}
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (c *Client) BuildWebLoginURL(state string) (string, error) {
	if strings.TrimSpace(c.config.AppID) == "" {
		return "", fmt.Errorf("wechat app_id is required")
	}
	redirectURI := strings.TrimSpace(c.config.WebRedirectURI)
	if redirectURI == "" {
		return "", fmt.Errorf("wechat web_redirect_uri is required")
	}
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("wechat state is required")
	}

	query := url.Values{}
	query.Set("appid", strings.TrimSpace(c.config.AppID))
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", c.loginScope())
	query.Set("state", state)

	return c.connectBaseURL() + "?" + query.Encode() + "#wechat_redirect", nil
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	values := url.Values{}
	values.Set("appid", strings.TrimSpace(c.config.AppID))
	values.Set("secret", strings.TrimSpace(c.config.AppSecret))
	values.Set("code", strings.TrimSpace(code))
	values.Set("grant_type", "authorization_code")

	body, err := c.callAPI(ctx, "/sns/oauth2/access_token", values)
	if err != nil {
		return nil, err
	}
	var resp OAuthToken
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if err := buildAPIError(resp.ErrCode, resp.ErrMsg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.AccessToken) == "" || strings.TrimSpace(resp.OpenID) == "" {
		return nil, fmt.Errorf("invalid oauth response: missing access_token/openid")
	}
	return &resp, nil
}

func (c *Client) ExchangeMiniProgramCode(ctx context.Context, code string) (*MiniProgramSession, error) {
	values := url.Values{}
	values.Set("appid", strings.TrimSpace(c.config.AppID))
	values.Set("secret", strings.TrimSpace(c.config.AppSecret))
	values.Set("js_code", strings.TrimSpace(code))
	values.Set("grant_type", "authorization_code")

	body, err := c.callAPI(ctx, "/sns/jscode2session", values)
	if err != nil {
		return nil, err
	}
	var resp MiniProgramSession
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if err := buildAPIError(resp.ErrCode, resp.ErrMsg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.OpenID) == "" || strings.TrimSpace(resp.SessionKey) == "" {
		return nil, fmt.Errorf("invalid mini program session response: missing openid/session_key")
	}
	return &resp, nil
}

func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*OAuthToken, error) {
	values := url.Values{}
	values.Set("appid", strings.TrimSpace(c.config.AppID))
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", strings.TrimSpace(refreshToken))

	body, err := c.callAPI(ctx, "/sns/oauth2/refresh_token", values)
	if err != nil {
		return nil, err
	}
	var resp OAuthToken
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if err := buildAPIError(resp.ErrCode, resp.ErrMsg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.AccessToken) == "" || strings.TrimSpace(resp.OpenID) == "" {
		return nil, fmt.Errorf("invalid refresh response: missing access_token/openid")
	}
	return &resp, nil
}

func (c *Client) VerifyAccessToken(ctx context.Context, accessToken, openID string) error {
	values := url.Values{}
	values.Set("access_token", strings.TrimSpace(accessToken))
	values.Set("openid", strings.TrimSpace(openID))

	body, err := c.callAPI(ctx, "/sns/auth", values)
	if err != nil {
		return err
	}
	var resp AuthCheckResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	return buildAPIError(resp.ErrCode, resp.ErrMsg)
}

func (c *Client) EnsureAccessTokenValid(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	if token == nil {
		return nil, fmt.Errorf("empty oauth token")
	}
	if err := c.VerifyAccessToken(ctx, token.AccessToken, token.OpenID); err == nil {
		return token, nil
	} else if !IsRetryableTokenError(err) || strings.TrimSpace(token.RefreshToken) == "" {
		return nil, err
	}

	refreshed, err := c.RefreshAccessToken(ctx, token.RefreshToken)
	if err != nil {
		return nil, err
	}
	if err := c.VerifyAccessToken(ctx, refreshed.AccessToken, refreshed.OpenID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(refreshed.UnionID) == "" {
		refreshed.UnionID = token.UnionID
	}
	return refreshed, nil
}

func (c *Client) FetchUserInfo(ctx context.Context, accessToken, openID string) (*UserInfo, error) {
	values := url.Values{}
	values.Set("access_token", strings.TrimSpace(accessToken))
	values.Set("openid", strings.TrimSpace(openID))
	values.Set("lang", "zh_CN")

	body, err := c.callAPI(ctx, "/sns/userinfo", values)
	if err != nil {
		return nil, err
	}
	var resp UserInfo
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if err := buildAPIError(resp.ErrCode, resp.ErrMsg); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) callAPI(ctx context.Context, endpoint string, values url.Values) ([]byte, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(c.config.APIBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}

	uri := fmt.Sprintf("%s/%s", baseURL, strings.TrimLeft(strings.TrimSpace(endpoint), "/"))
	if len(values) > 0 {
		uri += "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	observability.PropagateRequestID(req, ctx)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wechat http status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) connectBaseURL() string {
	baseURL := strings.TrimSpace(c.config.ConnectBaseURL)
	if baseURL == "" {
		return defaultConnectBaseURL
	}
	return baseURL
}

func (c *Client) loginScope() string {
	scope := strings.TrimSpace(c.config.LoginScope)
	if scope == "" {
		return defaultLoginScope
	}
	return scope
}

func buildAPIError(code int, message string) error {
	if code == 0 {
		return nil
	}
	return &APIError{Code: code, Message: strings.TrimSpace(message)}
}

func IsRetryableTokenError(err error) bool {
	var apiErr *APIError
	if !stdErrors.As(err, &apiErr) {
		return false
	}
	_, ok := retryableTokenErrorCodes[apiErr.Code]
	return ok
}
