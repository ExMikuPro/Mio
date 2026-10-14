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

// 测试函数调用的静态类型检查
func TestCheckFunctionCalls(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "正常函数调用",
			input: `
waza add(a: int64, b: int64) -> int64 {
    kaesu a + b;
}

kaku result = add(10, 20);
`,
		},
		{
			name: "函数提前调用",
			input: `
kaku result = add(10, 20);

waza add(a: int64, b: int64) -> int64 {
    kaesu a + b;
}
`,
		},
		{
			name: "嵌套函数调用",
			input: `
waza square(n: int64) -> int64 {
    kaesu n * n;
}

kaku result = square(square(2));
`,
		},
		{
			name: "递归函数调用",
			input: `
waza countdown(n: int64) -> int64 {
    moshi (n == 0) {
        kaesu 0;
    } sore {
        kaesu countdown(n - 1);
    }
}

kaku result = countdown(3);
`,
		},
		{
			name: "无参数函数",
			input: `
waza answer() -> int64 {
    kaesu 42;
}

kaku result = answer();
`,
		},
		{
			name: "实参数量不足",
			input: `
waza add(a: int64, b: int64) -> int64 {
    kaesu a + b;
}

kaku result = add(10);
`,
			wantErr: "expects 2 arguments, got 1",
		},
		{
			name: "实参数量过多",
			input: `
waza add(a: int64, b: int64) -> int64 {
    kaesu a + b;
}

kaku result = add(10, 20, 30);
`,
			wantErr: "expects 2 arguments, got 3",
		},
		{
			name: "实参类型不匹配",
			input: `
waza add(a: int64, b: int64) -> int64 {
    kaesu a + b;
}

kaku result = add(10, "Mio");
`,
			wantErr: "argument 2",
		},
		{
			name: "禁止调用普通变量",
			input: `
kaku age = 18;
age(10);
`,
			wantErr: "cannot call variable",
		},
		{
			name: "函数返回值参与布尔运算",
			input: `
waza positive(n: int64) -> bool {
    kaesu n > 0;
}

kaku result = positive(10) && hai;
`,
		},
		{
			name: "函数返回值类型不符合算术规则",
			input: `
waza positive(n: int64) -> bool {
    kaesu n > 0;
}

kaku result = positive(10) + 1;
`,
			wantErr: "requires int64 operands",
		},
		{
			name: "函数之间提前调用",
			input: `
waza first(n: int64) -> int64 {
    kaesu second(n);
}

waza second(n: int64) -> int64 {
    kaesu n;
}

kaku result = first(5);
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, r := parseResolvedSignatureProgram(
				t,
				tt.input,
			)

			checker := NewWithResolver(r)
			err := checker.CheckProgram(program)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf(
						"unexpected type error: %v",
						err,
					)
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

// 测试 chant 内置函数的类型检查
func TestCheckBuiltinChant(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:  "输出字符串",
			input: `chant("Hello Mio");`,
		},
		{
			name: "输出整数变量",
			input: `
kaku age = 18;
chant(age);
`,
		},
		{
			name:  "输出布尔表达式",
			input: `chant(10 > 5);`,
		},
		{
			name:  "输出空值",
			input: `chant(mu);`,
		},
		{
			name:    "缺少参数",
			input:   `chant();`,
			wantErr: "expects 1 argument, got 0",
		},
		{
			name:    "参数过多",
			input:   `chant(1, 2);`,
			wantErr: "expects 1 argument, got 2",
		},
		{
			name:    "不能将 unit 保存为变量",
			input:   `kaku result = chant(10);`,
			wantErr: `cannot infer type of "result" from unit`,
		},
		{
			name:    "不能将 unit 作为输出参数",
			input:   `chant(chant(10));`,
			wantErr: "cannot accept unit",
		},
		{
			name: "同名普通变量不能当作函数调用",
			input: `
kaku chant = 10;
chant(20);
`,
			wantErr: "cannot call variable",
		},
		{
			name: "用户函数遮蔽内置函数",
			input: `
waza chant(n: int64) -> int64 {
    kaesu n;
}

kaku result = chant(10);
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, r := parseResolvedSignatureProgram(
				t,
				tt.input,
			)

			checker := NewWithResolver(r)
			err := checker.CheckProgram(program)

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
