//go:build !linux

package core

import "errors"

var errIPSetUnsupported = errors.New("ipset firewall requires Linux; use mock for development")

type IpSetFirewallCore struct{ ipset, chain string }

func (*IpSetFirewallCore) InitRules() error                    { return errIPSetUnsupported }
func (*IpSetFirewallCore) Ban(string, string) error            { return errIPSetUnsupported }
func (*IpSetFirewallCore) RevertBan(string, string) error      { return errIPSetUnsupported }
func (*IpSetFirewallCore) Allow(string) error                  { return errIPSetUnsupported }
func (*IpSetFirewallCore) RevertAllow(string) error            { return errIPSetUnsupported }
func (*IpSetFirewallCore) ApplyRateLimit(RateLimitRule) error  { return errIPSetUnsupported }
func (*IpSetFirewallCore) RemoveRateLimit(RateLimitRule) error { return errIPSetUnsupported }
func (*IpSetFirewallCore) CleanupIpNetRules(string) error      { return errIPSetUnsupported }
func (*IpSetFirewallCore) CleanupRules() error                 { return errIPSetUnsupported }
