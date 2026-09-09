package config

import (
	"encoding/json"
	"testing"
)

func validConfig() *Config {
	cfg := DefaultConfig()
	cfg.Web.Auth.Enabled = false
	return cfg
}

func TestValidateDefault(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("default config should be valid, got: %v", err)
	}
}

func TestValidateFirewallType(t *testing.T) {
	cfg := validConfig()
	cfg.Firewall.Type = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid firewall type")
	}
}

func TestValidateUnsupportedDriver(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Driver = "mysql"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for mysql driver")
	}
}

func TestValidateBasicAuth(t *testing.T) {
	cfg := validConfig()
	cfg.Web.Auth.Enabled = true
	cfg.Web.Auth.Type = "basic"
	// 缺少用户名密码
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for incomplete basic auth")
	}

	cfg.Web.Auth.Basic.Username = "admin"
	cfg.Web.Auth.Basic.Password = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateOIDCAuth(t *testing.T) {
	cfg := validConfig()
	cfg.Web.Auth.Enabled = true
	cfg.Web.Auth.Type = "oidc"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for incomplete oidc config")
	}

	cfg.Web.Auth.OIDC.ClientID = "client"
	cfg.Web.Auth.OIDC.IssuerURL = "https://example.com"
	cfg.Web.Auth.OIDC.RedirectURL = "https://example.com/auth/callback"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateEmptyListen(t *testing.T) {
	cfg := validConfig()
	cfg.Web.Listen = " "
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty listen")
	}
}

func TestRedacted(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Password = "dbpass"
	cfg.Web.Auth.Enabled = true
	cfg.Web.Auth.Basic.Username = "admin"
	cfg.Web.Auth.Basic.Password = "secret"
	cfg.Web.Auth.OIDC.ClientSecret = "oidcsecret"

	redacted := cfg.Redacted()

	data, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"dbpass", "secret", "oidcsecret"} {
		if string(data) == "" {
			t.Fatal("empty json")
		}
		// 注意 "secret" 是 BasicAuth 密码，也是字段名，检查值形式
		if contains := containsValue(string(data), `"`+secret+`"`); contains {
			t.Fatalf("secret %q leaked in redacted config: %s", secret, data)
		}
	}

	// 原配置不受影响
	if cfg.Database.Password != "dbpass" || cfg.Web.Auth.Basic.Password != "secret" {
		t.Fatal("redacted() should not mutate original config")
	}
}

func containsValue(s, sub string) bool {
	return len(sub) > 0 && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
