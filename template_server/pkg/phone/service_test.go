package phone

import (
	"context"
	"testing"
	"time"
)

type memoryStore struct {
	values map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{values: map[string]string{}}
}

func (s *memoryStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	s.values[key] = value
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrInvalidCaptcha
	}
	return value, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type captureSender struct {
	message CaptchaMessage
}

func (s *captureSender) SendCaptcha(message CaptchaMessage) error {
	s.message = message
	return nil
}

func TestPhoneServiceSendAndVerifyKeepsScene(t *testing.T) {
	store := newMemoryStore()
	sender := &captureSender{}
	service := NewService(store, sender, Config{
		TTL:           5 * time.Minute,
		CaptchaLength: 4,
	})

	result, err := service.Send(context.Background(), "17608265580", "rebind")
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if sender.message.Scene != "rebind" {
		t.Fatalf("sender scene = %q, want rebind", sender.message.Scene)
	}
	if result.CaptchaKey == "" {
		t.Fatal("captcha key should not be empty")
	}

	err = service.Verify(context.Background(), VerifyRequest{
		Phone:      "17608265580",
		Captcha:    sender.message.Captcha,
		CaptchaKey: result.CaptchaKey,
		Scene:      "login",
	})
	if err == nil {
		t.Fatal("Verify should reject captcha with mismatched scene")
	}

	if err := service.Verify(context.Background(), VerifyRequest{
		Phone:      "17608265580",
		Captcha:    sender.message.Captcha,
		CaptchaKey: result.CaptchaKey,
		Scene:      "rebind",
	}); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}
