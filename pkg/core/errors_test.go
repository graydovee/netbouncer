package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotExistErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"iptables bad rule", errors.New("iptables: Bad rule (does a matching rule exist in that chain?)"), true},
		{"ipset not found", errors.New("ipset set netbouncer_ban not found"), true},
		{"truncated kernel message", errors.New("Element cannot be deleted from the set: exis"), true},
		{"element missing", errors.New("element missing"), true},
		{"other error", errors.New("permission denied"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotExistErr(tc.err); got != tc.want {
				t.Errorf("isNotExistErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsAlreadyExistsErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ipset already in set", errors.New("element already in set"), true},
		{"generic already exists", errors.New("rule already exists"), true},
		{"other error", errors.New("permission denied"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAlreadyExistsErr(tc.err); got != tc.want {
				t.Errorf("isAlreadyExistsErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestBuildIpSetEntry(t *testing.T) {
	cases := []struct {
		input    string
		wantIP   string
		wantCIDR uint8
		wantErr  bool
	}{
		{"192.168.1.1", "192.168.1.1", 32, false},
		{"192.168.0.0/16", "192.168.0.0", 16, false},
		{"2001:db8::1", "2001:db8::1", 128, false},
		{"not-an-ip", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			entry, err := buildIpSetEntry(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if entry.IP.String() != tc.wantIP {
				t.Errorf("IP = %s, want %s", entry.IP.String(), tc.wantIP)
			}
			if entry.CIDR != tc.wantCIDR {
				t.Errorf("CIDR = %d, want %d", entry.CIDR, tc.wantCIDR)
			}
		})
	}
}

// 确保 errors.Join 的行为符合 CleanupIpNetRules 的预期
func TestJoinedErrorsCheck(t *testing.T) {
	err := errors.Join(nil, fmt.Errorf("boom"))
	if err == nil {
		t.Fatal("expected non-nil joined error")
	}
	if !isNotExistErr(fmt.Errorf("wrapped: %w", errors.New("Bad rule"))) {
		t.Fatal("wrapped error should still be detected")
	}
}
