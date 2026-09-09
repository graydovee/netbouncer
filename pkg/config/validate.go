package config

import (
	"fmt"
	"strings"
)

// Validate 校验配置合法性，在启动时尽早发现配置错误
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Web.Listen) == "" {
		return fmt.Errorf("web.listen 不能为空")
	}

	switch FirewallType(c.Firewall.Type) {
	case FirewallTypeIptables, FirewallTypeIpSet, FirewallTypeMock:
	default:
		return fmt.Errorf("firewall.type 无效: %q (支持 iptables|ipset|mock)", c.Firewall.Type)
	}

	if c.Firewall.Type != string(FirewallTypeMock) && strings.TrimSpace(c.Firewall.Chain) == "" {
		return fmt.Errorf("firewall.chain 不能为空")
	}

	if c.Database.Driver != "sqlite" {
		return fmt.Errorf("database.driver 目前仅支持 sqlite，当前为 %q", c.Database.Driver)
	}

	if c.Web.Auth.Enabled {
		switch strings.ToLower(c.Web.Auth.Type) {
		case "", "basic":
		case "oidc":
			if c.Web.Auth.OIDC.ClientID == "" || c.Web.Auth.OIDC.IssuerURL == "" || c.Web.Auth.OIDC.RedirectURL == "" {
				return fmt.Errorf("启用 OIDC 认证时 web.auth.oidc 的 client_id、issuer_url、redirect_url 均不能为空")
			}
		default:
			return fmt.Errorf("web.auth.type 无效: %q (支持 basic|oidc)", c.Web.Auth.Type)
		}

		if (c.Web.Auth.Type == "" || c.Web.Auth.Type == "basic") &&
			(c.Web.Auth.Basic.Username == "" || c.Web.Auth.Basic.Password == "") {
			return fmt.Errorf("启用 BasicAuth 认证时 web.auth.basic 的 username 和 password 均不能为空")
		}
	}

	for _, rule := range c.Rules {
		switch rule.Action {
		case "ban", "allow":
		default:
			return fmt.Errorf("rules.action 无效: %q (支持 ban|allow)", rule.Action)
		}
	}

	return nil
}

// Redacted 返回脱敏后的配置副本，仅用于日志输出，避免泄露密码与密钥
func (c *Config) Redacted() *Config {
	copied := *c
	copied.Database.Password = maskSecret(c.Database.Password)
	copied.Web.Auth.Basic.Password = maskSecret(c.Web.Auth.Basic.Password)
	copied.Web.Auth.OIDC.ClientSecret = maskSecret(c.Web.Auth.OIDC.ClientSecret)
	copied.Web.Auth.OIDC.SessionSecret = maskSecret(c.Web.Auth.OIDC.SessionSecret)
	return &copied
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}
