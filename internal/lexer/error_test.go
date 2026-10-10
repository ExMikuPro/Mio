package lexer

import (
	"testing"

	"Mio/internal/token"
)

func TestNewChecked(t *testing.T) {
	// 正常 UTF-8 源码
	l, err := NewChecked(`kaku x = "你好";`, token.V01)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if l.NextToken().Type != token.Let {
		t.Fatal("expected LET token")
	}

	// 非法 UTF-8 源码
	_, err = NewChecked("A\n你\xff", token.V01)

	if err == nil {
		t.Fatal("expected invalid UTF-8 error")
	}

	lexErr, ok := err.(*LexicalError)
	if !ok {
		t.Fatalf("unexpected error type: %T", err)
	}

	if lexErr.Kind != ErrInvalidUTF8 {
		t.Fatalf("unexpected error kind: %s", lexErr.Kind)
	}

	want := token.Position{
		Offset: 5,
		Line:   2,
		Column: 2,
	}

	if lexErr.Position != want {
		t.Fatalf(
			"got %+v, want %+v",
			lexErr.Position,
			want,
		)
	}
}

func TestInvalidEscapeDiagnostic(t *testing.T) {
	input := `"hi\q"`

	l, err := NewChecked(input, token.V01)
	if err != nil {
		t.Fatal(err)
	}

	got := l.NextToken()

	// 1. 检查 Token 类型
	if got.Type != token.Illegal {
		t.Fatalf(
			"got %s, want ILLEGAL",
			got.Type,
		)
	}

	// 2. 检查 LexicalError
	lexErr, ok := l.Err().(*LexicalError)
	if !ok {
		t.Fatal("expected LexicalError")
	}

	// 3. 检查错误类别
	if lexErr.Kind != ErrInvalidEscape {
		t.Fatalf(
			"got %s, want InvalidEscape",
			lexErr.Kind,
		)
	}

	// 4. 检查错误位置
	want := token.Position{
		Offset: 3,
		Line:   1,
		Column: 4,
	}

	if lexErr.Position != want {
		t.Fatalf(
			"got %+v, want %+v",
			lexErr.Position,
			want,
		)
	}

	t.Log(lexErr)
}

func TestUnterminatedStringDiagnostic(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"EOF", `"Hello`},
		{"Newline", "\"Hello\nWorld\""},
		{"TrailingBackslash", `"Hello\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewChecked(tt.input, token.V01)
			if err != nil {
				t.Fatal(err)
			}

			got := l.NextToken()

			if got.Type != token.Illegal {
				t.Fatalf(
					"got %s, want ILLEGAL",
					got.Type,
				)
			}

			lexErr, ok := l.Err().(*LexicalError)
			if !ok {
				t.Fatal("expected LexicalError")
			}

			if lexErr.Kind != ErrUnterminatedString {
				t.Fatalf(
					"got %s, want UnterminatedString",
					lexErr.Kind,
				)
			}

			want := token.Position{
				Offset: 0,
				Line:   1,
				Column: 1,
			}

			if lexErr.Position != want {
				t.Fatalf(
					"got %+v, want %+v",
					lexErr.Position,
					want,
				)
			}
		})
	}
}

func TestUnterminatedBlockCommentDiagnostic(t *testing.T) {
	input := "\n  /* Hello Mio"

	l, err := NewChecked(input, token.V01)
	if err != nil {
		t.Fatal(err)
	}

	got := l.NextToken()

	// 检查非法 Token
	if got.Type != token.Illegal {
		t.Fatalf(
			"got %s, want ILLEGAL",
			got.Type,
		)
	}

	// 检查词法错误
	lexErr, ok := l.Err().(*LexicalError)
	if !ok {
		t.Fatal("expected LexicalError")
	}

	if lexErr.Kind != ErrUnterminatedBlockComment {
		t.Fatalf(
			"got %s, want UnterminatedBlockComment",
			lexErr.Kind,
		)
	}

	// 注释开始于第二行第三列
	want := token.Position{
		Offset: 3,
		Line:   2,
		Column: 3,
	}

	if lexErr.Position != want {
		t.Fatalf(
			"got %+v, want %+v",
			lexErr.Position,
			want,
		)
	}
}

func TestUnexpectedCharacterDiagnostic(t *testing.T) {
	input := "kaku x = @;"

	l, err := NewChecked(input, token.V01)
	if err != nil {
		t.Fatal(err)
	}

	// 跳过 kaku、x、=
	for i := 0; i < 3; i++ {
		l.NextToken()
	}

	// 读取 @
	got := l.NextToken()

	if got.Type != token.Illegal {
		t.Fatalf(
			"got %s, want ILLEGAL",
			got.Type,
		)
	}

	if got.Literal != "@" {
		t.Fatalf(
			"got %q, want @",
			got.Literal,
		)
	}

	// 获取词法错误
	lexErr, ok := l.Err().(*LexicalError)
	if !ok {
		t.Fatal("expected LexicalError")
	}

	// 检查错误类别
	if lexErr.Kind != ErrUnexpectedCharacter {
		t.Fatalf(
			"got %s, want UnexpectedCharacter",
			lexErr.Kind,
		)
	}

	// 检查源码位置
	want := token.Position{
		Offset: 9,
		Line:   1,
		Column: 10,
	}

	if lexErr.Position != want {
		t.Fatalf(
			"got %+v, want %+v",
			lexErr.Position,
			want,
		)
	}

	t.Log(lexErr)
}
