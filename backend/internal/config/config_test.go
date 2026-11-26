package config

import (
	"strings"
	"testing"
)

func production() Config {
	return Config{
		Env: "production", DatabaseURL: "postgres://app:secret@db.internal:5432/yetibyrash",
		WebDir: "/app/web", ShopURL: "https://yetibyrash.com", AdminOrigins: []string{"https://yetibyrash.com"},
		PaystackSecretKey: "sk_live_x", SMTPAddr: "smtp.resend.com:587", SMTPUsername: "resend", SMTPPassword: "p",
		MailFrom: "Yetibyrash <orders@yetibyrash.com>", MediaStorage: "r2", R2AccountID: "a", R2AccessKeyID: "k",
		R2SecretAccessKey: "s", R2Bucket: "photos", MediaPublicURL: "https://media.yetibyrash.com",
		TermiiAPIKey: "t", SentryDSN: "https://x@sentry.io/1", ProxyHops: 1,
	}
}

func TestCheckGoodProduction(t *testing.T) {
	problems, warnings := production().Check()
	if len(problems) != 0 || len(warnings) != 0 {
		t.Fatalf("problems %v, warnings %v", problems, warnings)
	}
}

func TestCheckCatchesUnsafeProduction(t *testing.T) {
	c := production()
	c.ShopURL = "http://yetibyrash.com"
	c.SMTPAddr = "localhost:1025"
	c.MediaPublicURL = ""
	c.PaystackSecretKey = "sk_test_x"
	c.DatabaseURL = "postgres://yetibyrash:yetibyrash@localhost:5432/yetibyrash"
	problems, warnings := c.Check()
	all := strings.Join(problems, "\n")
	for _, want := range []string{"SHOP_URL", "SMTP_ADDR", "MEDIA_PUBLIC_URL", "DATABASE_URL"} {
		if !strings.Contains(all, want) {
			t.Errorf("no problem about %s in:\n%s", want, all)
		}
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "TEST key") {
		t.Errorf("no warning about the Paystack test key: %v", warnings)
	}
}

func TestDevelopmentHasNoChecks(t *testing.T) {
	if p, w := (Config{Env: "development"}).Check(); p != nil || w != nil {
		t.Fatalf("development: %v %v", p, w)
	}
}
