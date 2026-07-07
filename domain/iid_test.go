package domain

import "testing"

func TestNormalizeIID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "  \t", want: ""},
		{name: "trim and uppercase", in: " kt-standard ", want: "KT-STANDARD"},
		{name: "full width ascii", in: "ＫＴ１２３", want: "KT123"},
		{name: "remove internal whitespace", in: "KT 板\t覆膜", want: "KT板覆膜"},
		{name: "full width space", in: "常规　KT", want: "常规KT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeIID(tt.in); got != tt.want {
				t.Fatalf("NormalizeIID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
