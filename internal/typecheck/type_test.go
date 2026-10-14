package typecheck

import "testing"

// 测试基础类型名称
func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindInt, "int64"},
		{KindBool, "bool"},
		{KindString, "string"},
		{KindNil, "nil"},
		{KindInvalid, "invalid"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Fatalf(
				"got %q, want %q",
				got,
				tt.want,
			)
		}
	}
}
