package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSourceCLI(t *testing.T, command, source string) (int, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.mio")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := runCLI([]string{command, path}, &stdout, &stderr)
	return exitCode, stdout.String(), stderr.String()
}

func TestCLIStringAndNil(t *testing.T) {
	tests := []struct{ source, want string }{
		{`"Hello, Mio!";`, "Hello, Mio!\n"},
		{`"";`, "\n"},
		{`"你好🌸\n\t\r\"\\";`, "你好🌸\n\t\r\"\\\n"},
		{`mu;`, "mu\n"},
		{`"Mio" == "Mio";`, "hai\n"},
		{`"Mio" != "Hello";`, "hai\n"},
		{`"Mio" == "Hello";`, "iie\n"},
		{`"Mio" != "Mio";`, "iie\n"},
		{`"Hello, Mio!"; mu; "Mio" == "Mio"; "Mio" != "Hello";`, "hai\n"},
		{`42; "Mio"; mu;`, "mu\n"},
		{`mu; "Mio"; 42;`, "42\n"},
		{`hai;`, "hai\n"},
		{`iie;`, "iie\n"},
		{`kaku s = "Mio"; s = "Hello"; s;`, "Hello\n"},
		{`kaku s = "Mio"; moshi (s == "Mio") { s = "Hello"; } s;`, "Hello\n"},
		{`moshi (iie) { "Mio"; } sore { mu; }`, "mu\n"},
		{`("Mio" == "Mio") || (1 / 0 == 0);`, "hai\n"},
		{`("Mio" != "Mio") && (1 / 0 == 0);`, "iie\n"},
		{`moshi (iie) { mu; }`, ""},
		{`kaku s = "Mio";`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			code, out, stderr := runSourceCLI(t, "run", tt.source)
			if code != 0 || stderr != "" || out != tt.want {
				t.Fatalf("exit=%d, stdout=%q, stderr=%q; want stdout=%q", code, out, stderr, tt.want)
			}
			code, out, stderr = runSourceCLI(t, "check", tt.source)
			if code != 0 || stderr != "" || out != "check passed\n" {
				t.Fatalf("check: exit=%d, stdout=%q, stderr=%q", code, out, stderr)
			}
		})
	}
}

func TestCLIStringAndNilStaticErrors(t *testing.T) {
	tests := []struct{ source, wantErr string }{
		{`"1" == 1;`, "requires matching types"},
		{`"hai" != hai;`, "requires matching types"},
		{`"" == mu;`, "requires matching types"},
		{`"a" + "b";`, "requires int64 operands"},
		{`"a" < "b";`, "requires int64 operands"},
		{`!"";`, "requires bool"},
		{`!mu;`, "requires bool"},
		{`"Mio" && hai;`, "requires bool operands"},
		{`moshi ("Mio") { mu; }`, "moshi condition requires bool"},
		{`moshi (mu) { 1; }`, "moshi condition requires bool"},
		{`mu == mu;`, "does not support nil operands"},
		{`mu != mu;`, "does not support nil operands"},
		{`kaku x = mu;`, `cannot infer type of "x" from nil`},
		{`kaku s = "Mio"; s = 1;`, "cannot assign int64"},
		{`kaku s = "Mio"; s = mu;`, "cannot assign nil"},
		{`kaku n = 1; n = "1";`, "cannot assign string"},
		{`kaku x = chant("Mio");`, `cannot infer type of "x" from unit`},
	}
	for _, tt := range tests {
		for _, command := range []string{"run", "check"} {
			t.Run(command+"/"+tt.source, func(t *testing.T) {
				code, out, stderr := runSourceCLI(t, command, tt.source)
				if code != 1 || out != "" || !strings.Contains(stderr, tt.wantErr) {
					t.Fatalf("exit=%d, stdout=%q, stderr=%q; want error %q", code, out, stderr, tt.wantErr)
				}
				if strings.Contains(stderr, "mio: compile:") || strings.Contains(stderr, "mio: runtime:") {
					t.Fatalf("expected static rejection, got %q", stderr)
				}
			})
		}
	}
}
