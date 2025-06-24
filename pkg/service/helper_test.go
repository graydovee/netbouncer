package service

import (
	"reflect"
	"testing"
)

func Test_extractIPsAndCIDRs(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test_space",
			args: args{
				text: "192.168.1.1/24 192.168.1.2 192.168.1.3/24",
			},
			want: []string{"192.168.1.1/24", "192.168.1.2", "192.168.1.3/24"},
		},
		{
			name: "test_comma",
			args: args{
				text: "192.168.1.1/24,192.168.1.2,192.168.1.3/24",
			},
			want: []string{"192.168.1.1/24", "192.168.1.2", "192.168.1.3/24"},
		},
		{
			name: "test_newline",
			args: args{
				text: `
				192.168.1.1/24
				192.168.1.2
				192.168.1.3/24
				`,
			},
			want: []string{"192.168.1.1/24", "192.168.1.2", "192.168.1.3/24"},
		},
		{
			name: "test_semicolon",
			args: args{
				text: `
				192.168.1.1/24;192.168.1.2;192.168.1.3/24
				`,
			},
			want: []string{"192.168.1.1/24", "192.168.1.2", "192.168.1.3/24"},
		},
		{
			name: "test_net",
			args: args{
				text: `
				0.0.0.0/0
				0.0.0.0
				1.1.1.1
				1.1.1.1/32
				10.0.0.0
				10.0.0.0/8
				192.168.1.1
				192.168.1.0/24
				172.16.0.1
				172.16.0.0/12
				`,
			},
			want: []string{"0.0.0.0/0", "0.0.0.0", "1.1.1.1", "1.1.1.1/32", "10.0.0.0", "10.0.0.0/8", "192.168.1.1", "192.168.1.0/24", "172.16.0.1", "172.16.0.0/12"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractIPsAndCIDRs(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractIPsAndCIDRs() = %v, want %v", got, tt.want)
			}
		})
	}
}
