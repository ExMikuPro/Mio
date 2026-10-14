package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIChant(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{`chant("Hello, Mio!"); chant(123); chant(hai); chant(iie); chant(mu);`, "Hello, Mio!\n123\nhai\niie\nmu\n"},
		{`chant("你好、日本語🌸"); chant("");`, "你好、日本語🌸\n\n"},
		{`((chant(mu)));`, "mu\n"},
		{`moshi (hai) { chant(1); } sore { chant(2); } moshi (iie) { chant(3); } sore { chant(4); }`, "1\n4\n"},
		{`moshi (iie) { chant(1); }`, ""},
		{`kaku n = 1; chant(n); n = 2; chant(n >= 2); chant(iie && (1 / 0 == 0)); chant(hai || (1 / 0 == 0)); n;`, "1\nhai\niie\nhai\n2\n"},
		{`chant(1); 42;`, "1\n42\n"},
		{`42; chant(1);`, "1\n42\n"},
		{`mu; chant("Mio");`, "Mio\nmu\n"},
		{`chant(1); kaku chant = 42; chant;`, "1\n42\n"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			for i := 0; i < 2; i++ {
				exit, out, stderr := runSourceCLI(t, "run", tt.source)
				if exit != 0 || out != tt.want || stderr != "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q; want %q", exit, out, stderr, tt.want)
				}
			}
			exit, out, stderr := runSourceCLI(t, "check", tt.source)
			if exit != 0 || out != "check passed\n" || stderr != "" {
				t.Fatalf("check: exit=%d stdout=%q stderr=%q", exit, out, stderr)
			}
		})
	}
}

func TestCLIChantSemanticErrors(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{`chant();`, "expects 1 argument"},
		{`chant(1, 2);`, "expects 1 argument"},
		{`chant(chant(1));`, "cannot accept unit"},
		{`kaku n = chant(1);`, "from unit"},
		{`kaku n = 1; n = chant(1);`, "cannot assign unit"},
		{`moshi (chant(1)) { 2; }`, "requires bool"},
		{`chant(1) + 1;`, "requires int64 operands"},
		{`chant(1) == chant(2);`, "does not support unit operands"},
		{`hai && chant(1);`, "requires bool operands"},
		{`kaku chant = 1; chant(2);`, "cannot call variable"},
		{`chant(missing);`, "undefined identifier"},
		{`chant("1" + 1);`, "requires int64 operands"},
		{`chant(mu == mu);`, "does not support nil operands"},
	} {
		for _, command := range []string{"check", "run"} {
			t.Run(command+tt.source, func(t *testing.T) {
				// 即使错误前有输出语句，整个程序也必须先完成语义检查。
				exit, out, stderr := runSourceCLI(t, command, `chant("before"); `+tt.source)
				if exit != 1 || out != "" || !strings.Contains(stderr, tt.want) {
					t.Fatalf("exit=%d stdout=%q stderr=%q; want %q", exit, out, stderr, tt.want)
				}
				if strings.Contains(stderr, "mio: compile:") || strings.Contains(stderr, "mio: runtime:") {
					t.Fatalf("not a semantic rejection: %q", stderr)
				}
			})
		}
	}
}

func TestCLIChantShadowedByFunction(t *testing.T) {
	for _, source := range []string{
		`chant(1); waza chant(n: int64) -> unit { kaesu; }`,
		`waza chant(n: int64) -> int64 { kaesu n; } chant(1);`,
		`moshi (hai) { chant(1); waza chant(n: int64) -> unit { kaesu; } }`,
	} {
		exit, out, stderr := runSourceCLI(t, "check", source)
		if exit != 0 || stderr != "" {
			t.Fatalf("shadowed function should pass semantic checks: %q", stderr)
		}
		exit, out, stderr = runSourceCLI(t, "run", source)
		if exit != 1 || out != "" || !strings.Contains(stderr, "mio: compile: user function") || !strings.Contains(stderr, "not supported yet") {
			t.Fatalf("exit=%d stdout=%q stderr=%q", exit, out, stderr)
		}
	}
}

type failingChantOutput struct{}

func (failingChantOutput) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestCLIChantOutputFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.mio")
	if err := os.WriteFile(path, []byte(`chant("Mio");`), 0644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	exit := runCLI([]string{"run", path}, failingChantOutput{}, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), "mio: runtime: OpChant at offset 3: output failed") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
}
