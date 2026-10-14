package compiler

import (
	"strings"
	"testing"
)

// 测试函数按声明顺序分配静态 FunctionID
func TestCompileFunctionIDsAndLocals(t *testing.T) {
	bytecode, err := compileGlobalSource(t, `
waza first(a: int64) -> int64 {
    kaku local = a + 1;
    kaesu second(local);
}
waza second(value: int64) -> int64 { kaesu value; }
first(1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytecode.Functions) != 2 {
		t.Fatalf("function count = %d, want 2", len(bytecode.Functions))
	}
	if bytecode.Functions[0].ID != 0 || bytecode.Functions[1].ID != 1 {
		t.Fatalf("function IDs: %#v", bytecode.Functions)
	}
	if bytecode.Functions[0].ParameterCount != 1 || bytecode.Functions[0].LocalCount != 2 {
		t.Fatalf("first metadata: %#v", bytecode.Functions[0])
	}
	disassembly := bytecode.Functions[0].Instructions.String()
	for _, want := range []string{"OpGetLocal 0", "OpSetLocal 1", "OpGetLocal 1", "OpCall 1", "OpReturnValue"} {
		if !strings.Contains(disassembly, want) {
			t.Fatalf("function instructions missing %q:\n%s", want, disassembly)
		}
	}
	if got := bytecode.Instructions.String(); !strings.Contains(got, "OpCall 0") || !strings.Contains(got, "OpPop") {
		t.Fatalf("main instructions:\n%s", got)
	}
}
