package phone

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidCaptcha = fmt.Errorf("invalid captcha")

type Store interface {
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

type Sender interface {
	SendCaptcha(phone string, expireMinutes int, captcha string) error
}

type Config struct {
	TestPhone      string
	TestCaptcha    string
	TestCaptchaKey string
	TTL            time.Duration
	CaptchaLength  int
}

type SendResult struct {
	CaptchaKey string
	ExpiresIn  int
}

type VerifyRequest struct {
	Phone      string
	Captcha    string
	CaptchaKey string
}

type Service struct {
	store  Store
	sender Sender
	config Config
}

func NewService(store Store, sender Sender, cfg Config) *Service {
	if cfg.TTL <= 0 {
		cfg.TTL = 5 * time.Minute
	}
	if cfg.CaptchaLength <= 0 {
		cfg.CaptchaLength = 4
	}
	return &Service{
		store:  store,
		sender: sender,
		config: cfg,
	}
}

func (s *Service) Send(ctx context.Context, phone string) (*SendResult, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil, fmt.Errorf("phone is required")
	}

	captchaKey := strings.TrimSpace(s.config.TestCaptchaKey)
	captcha := strings.TrimSpace(s.config.TestCaptcha)
	isTestPhone := strings.TrimSpace(s.config.TestPhone) != "" && phone == strings.TrimSpace(s.config.TestPhone)

	if !isTestPhone {
		var err error
		captcha, err = generateDigits(s.config.CaptchaLength)
		if err != nil {
			return nil, err
		}
		captchaKey = uuid.New().String()
	}

	payload, err := json.Marshal(map[string]string{
		"captchaKey": captchaKey,
		"captcha":    captcha,
	})
	if err != nil {
		return nil, err
	}

	if err := s.store.Set(ctx, phone, string(payload), s.config.TTL); err != nil {
		return nil, err
	}

	if !isTestPhone && s.sender != nil {
		if err := s.sender.SendCaptcha(phone, int(s.config.TTL/time.Minute), captcha); err != nil {
			return nil, err
		}
	}

	return &SendResult{
		CaptchaKey: captchaKey,
		ExpiresIn:  int(s.config.TTL / time.Second),
	}, nil
}

func (s *Service) Verify(ctx context.Context, req VerifyRequest) error {
	phone := strings.TrimSpace(req.Phone)
	if phone == "" {
		return ErrInvalidCaptcha
	}

	stored, err := s.store.Get(ctx, phone)
	if err != nil {
		return ErrInvalidCaptcha
	}

	payload, err := json.Marshal(map[string]string{
		"captchaKey": strings.TrimSpace(req.CaptchaKey),
		"captcha":    strings.TrimSpace(req.Captcha),
	})
	if err != nil {
		return err
	}

	if stored != string(payload) {
		return ErrInvalidCaptcha
	}

	isTestPhone := strings.TrimSpace(s.config.TestPhone) != "" && phone == strings.TrimSpace(s.config.TestPhone)
	if !isTestPhone {
		_ = s.store.Delete(ctx, phone)
	}

	return nil
}

func generateDigits(length int) (string, error) {
	if length <= 0 {
		length = 4
	}

	var builder strings.Builder
	builder.Grow(length)
	for range length {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + value.Int64()))
	}
	return builder.String(), nil
}
