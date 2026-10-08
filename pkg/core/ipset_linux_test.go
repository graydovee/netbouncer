//go:build linux

package core

import "testing"

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
