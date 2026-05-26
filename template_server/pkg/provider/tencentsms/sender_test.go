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

func TestResolveTemplateParamValues(t *testing.T) {
	values := resolveTemplateParamValues([]string{"captcha", "expire_minutes"}, authphone.CaptchaMessage{
		Captcha:       "1234",
		ExpireMinutes: 5,
	})
	if !reflect.DeepEqual(values, []string{"1234", "5"}) {
		t.Fatalf("values = %#v, want captcha and expire minutes", values)
	}
}
