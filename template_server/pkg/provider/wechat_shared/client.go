package wechatshared

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
	DefaultAPIBaseURL     = "https://api.weixin.qq.com"
	DefaultConnectBaseURL = "https://open.weixin.qq.com/connect/qrconnect"
	DefaultLoginScope     = "snsapi_login"
)

var RetryableTokenErrorCodes = map[int]struct{}{
	40001: {},
	40014: {},
	42001: {},
}

type BaseConfig struct {
	AppID                string
	AppSecret            string
	APIBaseURL           string
	RequestTimeoutSecond int
}

type WebConfig struct {
	BaseConfig
	ConnectBaseURL string
	WebRedirectURI string
	LoginScope     string
}

type Runtime struct {
	appID      string
	appSecret  string
	apiBaseURL string
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

func NewRuntime(cfg BaseConfig) *Runtime {
	timeout := cfg.RequestTimeoutSecond
	if timeout <= 0 {
		timeout = 5
	}
	return &Runtime{
		appID:      strings.TrimSpace(cfg.AppID),
		appSecret:  strings.TrimSpace(cfg.AppSecret),
		apiBaseURL: normalizeURL(cfg.APIBaseURL, DefaultAPIBaseURL),
		httpClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

func (r *Runtime) BuildOAuthURL(connectBaseURL string, redirectURI string, scope string, state string) (string, error) {
	if strings.TrimSpace(r.appID) == "" {
		return "", fmt.Errorf("wechat app_id is required")
	}
	if strings.TrimSpace(redirectURI) == "" {
		return "", fmt.Errorf("wechat web_redirect_uri is required")
	}
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("wechat state is required")
	}

	query := url.Values{}
	query.Set("appid", r.appID)
	query.Set("redirect_uri", strings.TrimSpace(redirectURI))
	query.Set("response_type", "code")
	query.Set("scope", fallbackString(scope, DefaultLoginScope))
	query.Set("state", strings.TrimSpace(state))
	return normalizeURL(connectBaseURL, DefaultConnectBaseURL) + "?" + query.Encode() + "#wechat_redirect", nil
}

func (r *Runtime) ExchangeOAuthCode(ctx context.Context, code string) (*OAuthToken, error) {
	values := url.Values{}
	values.Set("appid", r.appID)
	values.Set("secret", r.appSecret)
	values.Set("code", strings.TrimSpace(code))
	values.Set("grant_type", "authorization_code")

	body, err := r.callAPI(ctx, "/sns/oauth2/access_token", values)
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

func (r *Runtime) ExchangeMiniProgramCode(ctx context.Context, code string) (*MiniProgramSession, error) {
	values := url.Values{}
	values.Set("appid", r.appID)
	values.Set("secret", r.appSecret)
	values.Set("js_code", strings.TrimSpace(code))
	values.Set("grant_type", "authorization_code")

	body, err := r.callAPI(ctx, "/sns/jscode2session", values)
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

func (r *Runtime) RefreshAccessToken(ctx context.Context, refreshToken string) (*OAuthToken, error) {
	values := url.Values{}
	values.Set("appid", r.appID)
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", strings.TrimSpace(refreshToken))

	body, err := r.callAPI(ctx, "/sns/oauth2/refresh_token", values)
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

func (r *Runtime) VerifyAccessToken(ctx context.Context, accessToken string, openID string) error {
	values := url.Values{}
	values.Set("access_token", strings.TrimSpace(accessToken))
	values.Set("openid", strings.TrimSpace(openID))

	body, err := r.callAPI(ctx, "/sns/auth", values)
	if err != nil {
		return err
	}
	var resp AuthCheckResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	return buildAPIError(resp.ErrCode, resp.ErrMsg)
}

func (r *Runtime) FetchUserInfo(ctx context.Context, accessToken string, openID string) (*UserInfo, error) {
	values := url.Values{}
	values.Set("access_token", strings.TrimSpace(accessToken))
	values.Set("openid", strings.TrimSpace(openID))
	values.Set("lang", "zh_CN")

	body, err := r.callAPI(ctx, "/sns/userinfo", values)
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
	if strings.TrimSpace(resp.OpenID) == "" {
		return nil, fmt.Errorf("invalid user info response: missing openid")
	}
	return &resp, nil
}

func (r *Runtime) EnsureAccessTokenValid(ctx context.Context, token *OAuthToken) (*OAuthToken, error) {
	if token == nil {
		return nil, fmt.Errorf("empty oauth token")
	}
	if err := r.VerifyAccessToken(ctx, token.AccessToken, token.OpenID); err == nil {
		return token, nil
	} else {
		var apiErr *APIError
		if !stdErrors.As(err, &apiErr) || !IsRetryableTokenError(apiErr) {
			return nil, err
		}
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("missing refresh_token for wechat oauth token")
	}
	return r.RefreshAccessToken(ctx, token.RefreshToken)
}

func IsRetryableTokenError(err error) bool {
	var apiErr *APIError
	if !stdErrors.As(err, &apiErr) {
		return false
	}
	_, ok := RetryableTokenErrorCodes[apiErr.Code]
	return ok
}

func buildAPIError(code int, message string) error {
	if code == 0 {
		return nil
	}
	return &APIError{
		Code:    code,
		Message: strings.TrimSpace(message),
	}
}

func (r *Runtime) callAPI(ctx context.Context, path string, query url.Values) ([]byte, error) {
	endpoint := r.apiBaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	observability.PropagateRequestID(req, ctx)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wechat http status: %d", resp.StatusCode)
	}
	return body, nil
}

func normalizeURL(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = fallback
	}
	return strings.TrimRight(trimmed, "/")
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
