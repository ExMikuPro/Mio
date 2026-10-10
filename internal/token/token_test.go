package token

import "testing"

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		literal string
		version LanguageVersion
		want    Type
	}{
		{"kaku", V01, Let},
		{"waza", V01, Function},
		{"aida", V01, Ident},
		{"aida", V02, While},
		{"yobu", V02, Ident},
		{"yobu", V03, Import},
		{"chant", V01, Ident},
	}

	for _, tt := range tests {
		got := LookupIdent(tt.literal, tt.version)

		if got != tt.want {
			t.Errorf(
				"LookupIdent(%q) = %q, want %q",
				tt.literal,
				got,
				tt.want,
			)
		}
	}
}
