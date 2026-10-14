package vm

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
)

func TestVMChant(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{`chant("Hello, Mio!"); chant(123); chant(hai); chant(iie); chant(mu);`, "Hello, Mio!\n123\nhai\niie\nmu\n"},
		{`chant("你好🌸\n\t\"\\"); chant("");`, "你好🌸\n\t\"\\\n\n"},
		{`((chant(mu)));`, "mu\n"},
		{`moshi (hai) { chant(1); chant(2); } sore { chant(3); } chant(4);`, "1\n2\n4\n"},
		{`moshi (iie) { chant(1); } sore { chant(2); moshi (hai) { chant(3); } }`, "2\n3\n"},
		{`moshi (iie) { chant(1); }`, ""},
		{`kaku n = 1; chant(n); n = n + 1; chant(n); chant(n >= 2);`, "1\n2\nhai\n"},
		{`kaku s = "Mio"; chant(s == "Mio"); chant(iie && (1 / 0 == 0)); chant(hai || (1 / 0 == 0));`, "hai\niie\nhai\n"},
		{`chant(-9223372036854775807 - 1); chant(9223372036854775807);`, "-9223372036854775808\n9223372036854775807\n"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			var out bytes.Buffer
			machine := NewWithOutput(compileVariableSource(t, tt.source), &out)
			for run := 1; run <= 2; run++ {
				if err := machine.Run(); err != nil {
					t.Fatal(err)
				}
				if got := out.String(); got != strings.Repeat(tt.want, run) {
					t.Fatalf("run %d output=%q", run, got)
				}
				if len(machine.stack) != 0 {
					t.Fatalf("stack not empty: %v", machine.stack)
				}
				if _, ok := machine.LastPoppedValue(); ok {
					t.Fatal("unit call produced an expression result")
				}
			}
		})
	}
}

func TestVMChantPreservesOrdinaryResult(t *testing.T) {
	var out bytes.Buffer
	machine := NewWithOutput(compileVariableSource(t, `42; chant(1);`), &out)
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if got, ok := machine.LastPoppedValue(); !ok || got != value.NewInt(42) {
		t.Fatalf("result=%v, present=%v", got, ok)
	}
	// 新运行只执行 unit 调用，不能遗留上次普通表达式的结果。
	machine.bytecode = compileVariableSource(t, `chant(mu);`)
	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}
	if _, ok := machine.LastPoppedValue(); ok {
		t.Fatal("stale result from previous run")
	}
	if out.String() != "1\nmu\n" {
		t.Fatalf("output=%q", out.String())
	}
}

type chantErrorWriter struct {
	err   error
	calls int
}

func (w *chantErrorWriter) Write(p []byte) (int, error) { w.calls++; return 0, w.err }

type chantShortWriter struct{}

func (chantShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestVMChantOutputFailure(t *testing.T) {
	failure := errors.New("output unavailable")
	w := &chantErrorWriter{err: failure}
	for _, tt := range []struct {
		name string
		out  io.Writer
		want error
		text string
	}{
		{"writer error", w, failure, "output unavailable"},
		{"short write", chantShortWriter{}, io.ErrShortWrite, "short write"},
		{"nil writer", nil, nil, "missing output writer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machine := NewWithOutput(compileVariableSource(t, `chant(1); chant(2);`), tt.out)
			err := machine.Run()
			if err == nil || !strings.Contains(err.Error(), "OpChant at offset 3") || !strings.Contains(err.Error(), tt.text) {
				t.Fatalf("error=%v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("lost error cause: %v", err)
			}
			if len(machine.stack) != 0 {
				t.Fatal("failed chant did not consume argument")
			}
			if _, ok := machine.LastPoppedValue(); ok {
				t.Fatal("failed chant produced a result")
			}
			// 失败后恢复输出流，并从头再次执行。
			var out bytes.Buffer
			machine.output = &out
			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}
			if out.String() != "1\n2\n" || len(machine.stack) != 0 {
				t.Fatalf("retry output=%q, stack=%v", out.String(), machine.stack)
			}
		})
	}
	if w.calls != 1 {
		t.Fatalf("continued output after failure: %d writes", w.calls)
	}
}

func TestVMChantInvalidBytecode(t *testing.T) {
	for _, tt := range []struct {
		name         string
		instructions code.Instructions
		constants    []value.Value
		want         string
	}{
		{"empty stack", code.Instructions{byte(code.OpChant)}, nil, "stack underflow for OpChant at 0"},
		{"unit popped", code.Instructions{byte(code.OpNil), byte(code.OpChant), byte(code.OpPop)}, nil, "stack underflow for OpPop"},
		{"second chant", code.Instructions{byte(code.OpNil), byte(code.OpChant), byte(code.OpChant)}, nil, "stack underflow for OpChant at 2"},
		{"invalid constant", code.Instructions{byte(code.OpConstant), 0, 0, byte(code.OpChant)}, []value.Value{{}}, "invalid constant"},
		{"truncated argument", code.Instructions{byte(code.OpConstant), 0}, nil, "not enough bytes"},
		{"missing constant", code.Instructions{byte(code.OpConstant), 0, 0, byte(code.OpChant)}, nil, "constant index 0 out of range"},
		{"unknown opcode", code.Instructions{255, byte(code.OpChant)}, nil, "unknown opcode"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			machine := NewWithOutput(&compiler.Bytecode{Instructions: tt.instructions, Constants: tt.constants}, io.Discard)
			err := machine.Run()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestVMDefaultOutput(t *testing.T) {
	if New(&compiler.Bytecode{}).output != os.Stdout {
		t.Fatal("default output is not stdout")
	}
}
