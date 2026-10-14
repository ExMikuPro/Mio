package value

import "testing"

// 测试运行时值的类型标记
func TestValueKind(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		want  Kind
	}{
		{"整数", NewInt(123), KindInt},
		{"布尔真值", NewBool(true), KindBool},
		{"布尔假值", NewBool(false), KindBool},
		{"字符串", NewString("Mio"), KindString},
		{"空值", NewNil(), KindNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.Kind(); got != tt.want {
				t.Fatalf(
					"got %s, want %s",
					got,
					tt.want,
				)
			}
		})
	}
}

// 测试整数值读取
func TestIntValue(t *testing.T) {
	v := NewInt(123)

	number, ok := v.Int64()
	if !ok || number != 123 {
		t.Fatalf(
			"got %d, ok=%v, want 123",
			number,
			ok,
		)
	}

	// 布尔值不能按整数读取
	if _, ok := NewBool(true).Int64(); ok {
		t.Fatal("expected integer type mismatch")
	}
}

// 测试布尔值读取
func TestBoolValue(t *testing.T) {
	tests := []struct {
		input bool
	}{
		{true},
		{false},
	}

	for _, tt := range tests {
		got, ok := NewBool(tt.input).Bool()

		if !ok || got != tt.input {
			t.Fatalf(
				"got %v, ok=%v, want %v",
				got,
				ok,
				tt.input,
			)
		}
	}

	// 整数不能按布尔值读取
	if _, ok := NewInt(0).Bool(); ok {
		t.Fatal("expected boolean type mismatch")
	}
}

// 测试字符串与空值
func TestStringAndNilValue(t *testing.T) {
	str := NewString("Hello Mio")

	got, ok := str.Text()
	if !ok || got != "Hello Mio" {
		t.Fatalf(
			"got %q, ok=%v",
			got,
			ok,
		)
	}

	if !NewNil().IsNil() {
		t.Fatal("expected nil value")
	}

	if NewString("").IsNil() {
		t.Fatal("empty string must not be nil")
	}

	if _, ok := NewNil().Text(); ok {
		t.Fatal("nil must not be readable as string")
	}
}

// 测试零值不能冒充合法类型
func TestZeroValue(t *testing.T) {
	var v Value

	if v.Kind() != KindInvalid {
		t.Fatalf(
			"got %s, want invalid",
			v.Kind(),
		)
	}

	if _, ok := v.Int64(); ok {
		t.Fatal("invalid value must not be readable as int64")
	}
}
