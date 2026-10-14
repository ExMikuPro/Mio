package typecheck

import (
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/resolver"
	"Mio/internal/token"
	"strings"
	"testing"
)

// 测试条件语句与局部变量的类型检查
func TestCheckIfStatements(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "布尔变量作为条件",
			input: `
kaku ready = hai;

moshi (ready) {
    kaku count = 1;
    count = 2;
}
`,
		},
		{
			name: "比较表达式作为条件",
			input: `
kaku age = 18;

moshi (age >= 18) {
    age = 20;
} sore {
    age = 16;
}
`,
		},
		{
			name: "整数不能作为条件",
			input: `
moshi (123) {
    kaku value = 10;
}
`,
			wantErr: "moshi condition requires bool, got int64",
		},
		{
			name: "字符串不能作为条件",
			input: `
moshi ("Mio") {
    kaku value = 10;
}
`,
			wantErr: "moshi condition requires bool, got string",
		},
		{
			name: "条件内部存在类型错误",
			input: `
moshi (10 && hai) {
    kaku value = 10;
}
`,
			wantErr: "requires bool operands",
		},
		{
			name: "检查 sore 分支中的非法赋值",
			input: `
kaku value = 10;

moshi (hai) {
    value = 20;
} sore {
    value = "Mio";
}
`,
			wantErr: "cannot assign string",
		},
		{
			name: "不同分支允许同名变量拥有不同类型",
			input: `
moshi (hai) {
    kaku value = 10;
    value = 20;
} sore {
    kaku value = "Mio";
    value = "Hello";
}
`,
		},
		{
			name: "局部变量遮蔽不影响外层类型",
			input: `
kaku value = 10;

moshi (hai) {
    kaku value = "Mio";
    value = "Hello";
}

value = 20;
`,
		},
		{
			name: "内层初始化表达式引用外层变量",
			input: `
kaku value = 10;

moshi (hai) {
    kaku value = value + 1;
    value = 20;
}
`,
		},
		{
			name: "嵌套条件语句",
			input: `
moshi (hai) {
    moshi (iie) {
        kaku value = 10;
        value = 20;
    }
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
			if err != nil {
				t.Fatal(err)
			}

			p := parser.New(l)
			program, err := p.ParseProgram()
			if err != nil {
				t.Fatal(err)
			}

			r := resolver.New()
			if err := r.ResolveProgram(program); err != nil {
				t.Fatalf("unexpected resolver error: %v", err)
			}

			checker := NewWithResolver(r)
			err = checker.CheckProgram(program)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected type error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected type error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf(
					"expected error containing %q, got %q",
					tt.wantErr,
					err.Error(),
				)
			}
		})
	}
}
