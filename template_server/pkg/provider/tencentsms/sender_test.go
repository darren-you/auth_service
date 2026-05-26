package tencentsms

import (
	"reflect"
	"testing"

	authphone "github.com/darren-you/auth_service/template_server/pkg/phone"
)

func TestResolveTemplateUsesSceneSpecificParams(t *testing.T) {
	sender := NewSender(Config{
		TemplateID: "default-template",
		Templates: map[string]TemplateConfig{
			"login": {
				TemplateID: "2650973",
				Params:     []string{"captcha", "expire_minutes"},
			},
			"bind": {
				TemplateID: "2650974",
				Params:     []string{"captcha"},
			},
		},
	})

	templateID, params := sender.resolveTemplate("rebind")
	if templateID != "2650974" {
		t.Fatalf("templateID = %q, want 2650974", templateID)
	}
	if !reflect.DeepEqual(params, []string{"captcha"}) {
		t.Fatalf("params = %#v, want captcha only", params)
	}
}

func TestAppKeySignature(t *testing.T) {
	got := appKeySignature("5f03a35d00ee52a21327ab048186a2c4", "7226249334", 1457336869, "13788888888")
	want := "ecab4881ee80ad3d76bb1da68387428ca752eb885e52621a3129dcf4d9bc4fd4"
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestNormalizeMainlandMobile(t *testing.T) {
	cases := map[string]string{
		"17608265580":    "17608265580",
		"+8617608265580": "17608265580",
		"8617608265580":  "17608265580",
	}
	for input, want := range cases {
		if got := normalizeMainlandMobile(input); got != want {
			t.Fatalf("normalizeMainlandMobile(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveTemplateParamValues(t *testing.T) {
	values := resolveTemplateParamValues([]string{"captcha", "expire_minutes"}, authphone.CaptchaMessage{
		Captcha:       "1234",
		ExpireMinutes: 5,
	})
	if !reflect.DeepEqual(values, []string{"1234", "5"}) {
		t.Fatalf("values = %#v, want captcha and expire minutes", values)
	}
}
