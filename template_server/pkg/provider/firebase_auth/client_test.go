package firebaseauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientUsesStableJWKSTimeoutsAndTransport(t *testing.T) {
	client, err := NewClient(Config{ProjectID: "tinytext-global"})
	if err != nil {
		t.Fatalf("expected client to be created, got %v", err)
	}

	if client.httpClient.Timeout != defaultRequestTimeout {
		t.Fatalf("expected default timeout %s, got %s", defaultRequestTimeout, client.httpClient.Timeout)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.httpClient.Transport)
	}
	if transport.DialContext == nil {
		t.Fatalf("expected custom dial context")
	}
	if transport.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
		t.Fatalf("expected response header timeout %s, got %s", defaultResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	}
	if transport.TLSNextProto == nil {
		t.Fatalf("expected http2 to be disabled for jwks requests")
	}
}

func TestNewClientAcceptsConfiguredRequestTimeout(t *testing.T) {
	client, err := NewClient(Config{
		ProjectID:            "tinytext-global",
		RequestTimeoutSecond: 6,
	})
	if err != nil {
		t.Fatalf("expected client to be created, got %v", err)
	}

	if client.httpClient.Timeout != 6*time.Second {
		t.Fatalf("expected configured timeout, got %s", client.httpClient.Timeout)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http transport, got %T", client.httpClient.Transport)
	}
	if transport.ResponseHeaderTimeout != 6*time.Second {
		t.Fatalf("expected bounded response header timeout, got %s", transport.ResponseHeaderTimeout)
	}
}

func TestLoadPublicKeysCachesJWKSResponse(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", r.Method)
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		if err := json.NewEncoder(w).Encode(jwksDocument{
			Keys: []jwkKey{testJWK("firebase-test-key")},
		}); err != nil {
			t.Fatalf("encode jwks response failed: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		ProjectID: "tinytext-global",
		JWKSURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("expected client to be created, got %v", err)
	}

	keys, err := client.loadPublicKeys(context.Background())
	if err != nil {
		t.Fatalf("expected public keys to load, got %v", err)
	}
	publicKey := keys["firebase-test-key"]
	if publicKey == nil {
		t.Fatalf("expected firebase-test-key to be present")
	}
	if publicKey.E != 65537 {
		t.Fatalf("expected exponent 65537, got %d", publicKey.E)
	}

	if _, err := client.loadPublicKeys(context.Background()); err != nil {
		t.Fatalf("expected cached public keys to load, got %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected jwks response to be cached, got %d hits", hits)
	}
}

func testJWK(keyID string) jwkKey {
	return jwkKey{
		KeyID:     keyID,
		KeyType:   "RSA",
		Algorithm: "RS256",
		Use:       "sig",
		Modulus:   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04, 0x05}),
		Exponent:  base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}
}
