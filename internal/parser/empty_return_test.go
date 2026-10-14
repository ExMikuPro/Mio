package parser

import (
	"testing"

	"Mio/internal/ast"
	"Mio/internal/lexer"
	"Mio/internal/token"
)

// 测试有值和无值返回语句的解析
func TestParseEmptyReturn(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{
			name: "无值返回",
			input: `
waza example() -> unit {
    kaesu;
}
`,
			wantNil: true,
		},
		{
			name: "整数返回",
			input: `
waza example() -> int64 {
    kaesu 10;
}
`,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
			if err != nil {
				t.Fatal(err)
			}

			p := New(l)

			program, err := p.ParseProgram()
			if err != nil {
				t.Fatal(err)
			}

			fn, ok := program.Statements[0].(*ast.FunctionStatement)
			if !ok {
				t.Fatal("expected FunctionStatement")
			}

			ret, ok := fn.Body.Statements[0].(*ast.ReturnStatement)
			if !ok {
				t.Fatal("expected ReturnStatement")
			}

			if (ret.Value == nil) != tt.wantNil {
				t.Fatalf(
					"unexpected return value: %T",
					ret.Value,
				)
			}
		})
	}
}
