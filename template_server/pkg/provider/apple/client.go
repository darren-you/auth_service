package apple

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"os"

	signinapple "github.com/Timothylock/go-signin-with-apple/apple"
)

type Config struct {
	SigningKey string `json:"signing_key"`
	TeamID     string `json:"team_id"`
	ClientID   string `json:"client_id"`
	KeyID      string `json:"key_id"`
}

type Client struct {
	config      Config
	appleClient *signinapple.Client
}

type ValidationResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

func NewClient(cfg Config) *Client {
	return &Client{
		config:      cfg,
		appleClient: signinapple.New(),
	}
}

func NewClientWithSecretFile(filePath string) (*Client, error) {
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read apple secret file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, fmt.Errorf("decode apple secret file: %w", err)
	}
	return NewClient(cfg), nil
}

func (c *Client) VerifyAuthorizationCode(ctx context.Context, code string) (*ValidationResponse, error) {
	secret, err := signinapple.GenerateClientSecret(
		c.config.SigningKey,
		c.config.TeamID,
		c.config.ClientID,
		c.config.KeyID,
	)
	if err != nil {
		return nil, fmt.Errorf("generate apple client secret: %w", err)
	}

	req := signinapple.AppValidationTokenRequest{
		ClientID:     c.config.ClientID,
		ClientSecret: secret,
		Code:         code,
	}

	resp := &ValidationResponse{}
	if err := c.appleClient.VerifyAppToken(ctx, req, resp); err != nil {
		return nil, fmt.Errorf("verify apple authorization code: %w", err)
	}
	return resp, nil
}

func (c *Client) GetUniqueIDFromIDToken(idToken string) (string, error) {
	uniqueID, err := signinapple.GetUniqueID(idToken)
	if err != nil {
		return "", fmt.Errorf("get unique id from apple id_token: %w", err)
	}
	return uniqueID, nil
}

func IsAuthorizationError(err error) bool {
	if err == nil {
		return false
	}
	return !stdErrors.Is(err, os.ErrNotExist)
}
