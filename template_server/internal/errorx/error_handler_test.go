package errorx

import (
	"context"
	"testing"

	"github.com/darren-you/auth_service/template_server/pkg/responsex"
)

func TestHandlerReturnsWrappedCustomErrorMessage(t *testing.T) {
	status, payload := Handler(context.Background(), New(5000, 500, "internal server error", assertErr("update auth user login failed")))

	if status != 500 {
		t.Fatalf("expected status 500, got %d", status)
	}

	resp, ok := payload.(responsex.Envelope)
	if !ok {
		t.Fatalf("expected response envelope, got %T", payload)
	}

	if got := resp.Msg; got != "internal server error: update auth user login failed" {
		t.Fatalf("unexpected message: %v", got)
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
