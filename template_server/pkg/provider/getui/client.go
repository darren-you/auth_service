package getui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/observability"
)

const defaultBaseURL = "https://h-gy.getui.net/v2/gy/ct_login/gy_get_pn"

type Config struct {
	AppID        string `json:"app_id"`
	AppKey       string `json:"app_key"`
	AppSecret    string `json:"app_secret"`
	MasterSecret string `json:"master_secret"`
	BaseURL      string `json:"base_url"`
}

type responseEnvelope[T any] struct {
	Result string `json:"result"`
	Msg    string `json:"msg"`
	Data   T      `json:"data"`
}

type baseResponse[T any] struct {
	Errno int                 `json:"errno"`
	Data  responseEnvelope[T] `json:"data"`
}

type oneClickAuth struct {
	Pn string `json:"pn"`
}

type Client struct {
	config     Config
	httpClient *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) OneClickLogin(ctx context.Context, clientToken string, gyuid string) (string, error) {
	clientToken = strings.TrimSpace(clientToken)
	gyuid = strings.TrimSpace(gyuid)
	if clientToken == "" || gyuid == "" {
		return "", errors.New("invalid client token or gyuid")
	}

	timestamp := time.Now().UnixMilli()
	payload := map[string]any{
		"appId":     strings.TrimSpace(c.config.AppID),
		"gyuid":     gyuid,
		"timestamp": timestamp,
		"token":     clientToken,
		"sign":      GenSign(strings.TrimSpace(c.config.AppSecret), strconv.FormatInt(timestamp, 10), strings.TrimSpace(c.config.MasterSecret)),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal getui request: %w", err)
	}

	baseURL := strings.TrimSpace(c.config.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build getui request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	observability.PropagateRequestID(req, ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request getui failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed baseResponse[oneClickAuth]
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode getui response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("getui http status: %d", resp.StatusCode)
	}
	if parsed.Errno != 0 {
		return "", fmt.Errorf("getui errno=%d msg=%s", parsed.Errno, strings.TrimSpace(parsed.Data.Msg))
	}
	if parsed.Data.Result != "20000" || strings.TrimSpace(parsed.Data.Data.Pn) == "" {
		msg := strings.TrimSpace(parsed.Data.Msg)
		if msg == "" {
			msg = parsed.Data.Result
		}
		return "", fmt.Errorf("getui one click login failed: %s", msg)
	}

	phone, err := Decrypt(parsed.Data.Data.Pn, strings.TrimSpace(c.config.MasterSecret))
	if err != nil {
		return "", fmt.Errorf("decrypt getui phone: %w", err)
	}
	return strings.TrimSpace(phone), nil
}
