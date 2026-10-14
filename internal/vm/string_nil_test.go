package vm

import (
	"strings"
	"testing"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
)

func TestVMStringAndNil(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   value.Value
	}{
		{"字符串", `"Hello, Mio!";`, value.NewString("Hello, Mio!")},
		{"空字符串", `"";`, value.NewString("")},
		{"Unicode 与转义", `"你好🌸\n\t\r\"\\";`, value.NewString("你好🌸\n\t\r\"\\")},
		{"空值", `mu;`, value.NewNil()},
		{"重复空值", `mu; mu;`, value.NewNil()},
		{"空值覆盖整数结果", `42; mu;`, value.NewNil()},
		{"空值之后的整数", `mu; "Mio"; 42;`, value.NewInt(42)},
		{"相等为真", `"Mio" == "Mio";`, value.NewBool(true)},
		{"相等为假", `"Mio" == "Hello";`, value.NewBool(false)},
		{"不等为真", `"Mio" != "Hello";`, value.NewBool(true)},
		{"不等为假", `"Mio" != "Mio";`, value.NewBool(false)},
		{"空字符串相等", `"" == "";`, value.NewBool(true)},
		{"Unicode 比较", `"你好" == "你好";`, value.NewBool(true)},
		{"区分大小写", `"Mio" == "mio";`, value.NewBool(false)},
		{"转义与普通文本不同", `"\n" != "\\n";`, value.NewBool(true)},
		{"全局变量赋值", `kaku s = "Mio"; s = "Hello"; s;`, value.NewString("Hello")},
		{"全局变量比较", `kaku a = "Mio"; kaku b = "Mio"; a == b;`, value.NewBool(true)},
		{"字符串分支", `kaku s = "Mio"; moshi (s == "Mio") { s = "Hello"; } sore { s = "bad"; } s;`, value.NewString("Hello")},
		{"空值分支", `moshi ("Mio" != "Mio") { "bad"; } sore { mu; }`, value.NewNil()},
		{"与短路", `("Mio" != "Mio") && (1 / 0 == 0);`, value.NewBool(false)},
		{"或短路", `("Mio" == "Mio") || (1 / 0 == 0);`, value.NewBool(true)},
		{"与执行右侧", `hai && ("Mio" == "Mio");`, value.NewBool(true)},
		{"或执行右侧", `iie || ("Mio" != "Hello");`, value.NewBool(true)},
		{"完整示例", `"Hello, Mio!"; mu; "Mio" == "Mio"; "Mio" != "Hello";`, value.NewBool(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := New(compileVariableSource(t, tt.source))
			// 重复运行应重新初始化状态。
			for i := 0; i < 2; i++ {
				if err := machine.Run(); err != nil {
					t.Fatal(err)
				}
				got, ok := machine.LastPoppedValue()
				if !ok || got != tt.want {
					t.Fatalf("got %v, present=%v, want %v", got, ok, tt.want)
				}
				if len(machine.stack) != 0 {
					t.Fatalf("stack not empty: %v", machine.stack)
				}
				legacy, valid := machine.LastPoppedStackElem()
				wantInt, wantValid := tt.want.Int64()
				if valid != wantValid || legacy != wantInt {
					t.Fatalf("legacy result: %d, valid=%v", legacy, valid)
				}
			}
		})
	}
}

func TestVMNilResultPresence(t *testing.T) {
	machine := New(compileVariableSource(t, `mu;`))
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if got, ok := machine.LastPoppedValue(); !ok || !got.IsNil() {
		t.Fatal("mu must be a present result")
	}
	machine.bytecode = compileVariableSource(t, `moshi (iie) { mu; }`)
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if _, ok := machine.LastPoppedValue(); ok {
		t.Fatal("skipped expression must not produce a result")
	}
}

func TestVMStringAndNilTypeErrors(t *testing.T) {
	tests := []struct{ source, wantErr string }{
		{`"1" == 1;`, "requires matching types"},
		{`"hai" != hai;`, "requires matching types"},
		{`mu == "";`, "requires matching types"},
		{`mu == mu;`, "does not support nil operands"},
		{`mu != mu;`, "does not support nil operands"},
		{`"a" < "b";`, "requires int64 operands"},
		{`"a" + "b";`, "requires int64"},
		{`1 + mu;`, "requires int64"},
		{`-"1";`, "requires int64"},
		{`!mu;`, "requires bool"},
		{`!"";`, "requires bool"},
		{`moshi (mu) { 1; }`, "requires bool"},
		{`moshi ("Mio") { 1; }`, "requires bool"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			// 绕过类型检查，验证 VM 仍拒绝非法操作。
			err := New(compileSource(t, tt.source)).Run()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestVMInvalidTypedConstant(t *testing.T) {
	machine := New(&compiler.Bytecode{
		Constants:    []value.Value{{}},
		Instructions: code.Instructions{byte(code.OpConstant), 0, 0},
	})
	if err := machine.Run(); err == nil || !strings.Contains(err.Error(), "invalid constant") {
		t.Fatalf("expected invalid constant error, got %v", err)
	}
	if len(machine.stack) != 0 {
		t.Fatal("invalid constant entered stack")
	}
}
