package collector

import "testing"

func TestCapText(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"empty", "", 5, ""},
		{"shorter than the limit", "abc", 5, "abc"},
		{"exactly the limit", "abc", 3, "abc"},
		{"cut", "abcdef", 3, "abc"},
		{"two-byte rune at the cut", "abé", 3, "ab"},
		{"two-byte rune fits", "abé", 4, "abé"},
		{"four-byte rune at the cut", "a😀b", 3, "a"},
		{"zero limit", "abc", 0, ""},
		{"negative limit", "abc", -1, ""},
		{"invalid UTF-8 is cut by bytes", "a\xff\xfe\xfd", 2, "a\xff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CapText(tt.in, tt.limit); got != tt.want {
				t.Errorf("CapText(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
			}
		})
	}
}
