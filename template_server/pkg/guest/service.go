package guest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	Set(ctx context.Context, deviceID string, expiration time.Duration) error
	Exists(ctx context.Context, deviceID string) (bool, error)
}

type DeviceIDResponse struct {
	DeviceID  string
	ExpiresIn int
}

type Service struct {
	store Store
	ttl   time.Duration
}

func NewService(store Store, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Service{
		store: store,
		ttl:   ttl,
	}
}

func (s *Service) IssueVirtualDeviceID(ctx context.Context) (*DeviceIDResponse, error) {
	deviceID := uuid.New().String()
	if err := s.store.Set(ctx, deviceID, s.ttl); err != nil {
		return nil, err
	}
	return &DeviceIDResponse{
		DeviceID:  deviceID,
		ExpiresIn: int(s.ttl / time.Second),
	}, nil
}

func (s *Service) IsValid(ctx context.Context, deviceID string) (bool, error) {
	return s.store.Exists(ctx, deviceID)
}

func UsernameFromDeviceID(deviceID string) string {
	hash := sha256.Sum256([]byte(deviceID))
	return "游客_" + hex.EncodeToString(hash[:4])
}
