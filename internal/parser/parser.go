package parser

import (
	"fmt"
	"strconv"

	"Mio/internal/ast"
	"Mio/internal/lexer"
	"Mio/internal/token"
)

// 前缀解析函数：从当前 Token 开始解析表达式。
type prefixParseFn func() (ast.Expression, error)

// 中缀解析函数：接收已经解析的左操作数。
type infixParseFn func(ast.Expression) (ast.Expression, error)

// Parser 使用双 Token 前瞻与 Pratt 算法构建 AST。
type Parser struct {
	l *lexer.Lexer

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.Type]prefixParseFn
	infixParseFns  map[token.Type]infixParseFn
}

// New 创建解析器，并注册表达式对应的解析函数。
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:              l,
		prefixParseFns: make(map[token.Type]prefixParseFn),
		infixParseFns:  make(map[token.Type]infixParseFn),
	}

	// 基础表达式
	p.registerPrefix(token.Integer, p.parseIntegerLiteral)
	p.registerPrefix(token.String, p.parseStringLiteral)
	p.registerPrefix(token.True, p.parseBooleanLiteral)
	p.registerPrefix(token.False, p.parseBooleanLiteral)
	p.registerPrefix(token.Nil, p.parseNilLiteral)
	p.registerPrefix(token.Ident, p.parseIdentifier)

	// 一元运算与括号分组
	p.registerPrefix(token.Minus, p.parsePrefixExpression)
	p.registerPrefix(token.Bang, p.parsePrefixExpression)
	p.registerPrefix(token.LParen, p.parseGroupedExpression)

	// 算术运算
	p.registerInfix(token.Plus, p.parseInfixExpression)
	p.registerInfix(token.Minus, p.parseInfixExpression)
	p.registerInfix(token.Asterisk, p.parseInfixExpression)
	p.registerInfix(token.Slash, p.parseInfixExpression)

	// 比较运算
	p.registerInfix(token.Equal, p.parseInfixExpression)
	p.registerInfix(token.NotEqual, p.parseInfixExpression)
	p.registerInfix(token.Less, p.parseInfixExpression)
	p.registerInfix(token.LessEqual, p.parseInfixExpression)
	p.registerInfix(token.Greater, p.parseInfixExpression)
	p.registerInfix(token.GreaterEqual, p.parseInfixExpression)

	// 逻辑运算
	p.registerInfix(token.And, p.parseInfixExpression)
	p.registerInfix(token.Or, p.parseInfixExpression)

	// 函数调用：左侧已经存在表达式时，( 表示调用而非分组。
	p.registerInfix(token.LParen, p.parseCallExpression)

	// 初始化当前 Token 和前瞻 Token。
	p.nextToken()
	p.nextToken()

	return p
}

// nextToken 将前瞻 Token 移入当前位置，并读取下一个 Token。
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// registerPrefix 注册表达式起始 Token 的解析函数。
func (p *Parser) registerPrefix(tokenType token.Type, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix 注册已有左操作数时的解析函数。
func (p *Parser) registerInfix(tokenType token.Type, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// ParseProgram 解析整个程序，并记录源码范围。
func (p *Parser) ParseProgram() (*ast.Program, error) {
	program := &ast.Program{}

	for p.curToken.Type != token.EOF {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		program.Statements = append(program.Statements, stmt)
		p.nextToken()
	}

	if len(program.Statements) > 0 {
		first := program.Statements[0]
		last := program.Statements[len(program.Statements)-1]

		program.Range = ast.Span{
			Start: first.Span().Start,
			End:   last.Span().End,
		}
	} else {
		program.Range = ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		}
	}

	return program, nil
}

// parseStatement 根据当前 Token 分发语句解析。
func (p *Parser) parseStatement() (ast.Statement, error) {
	switch p.curToken.Type {
	case token.Let:
		return p.parseLetStatement()

	case token.Function:
		return p.parseFunctionStatement()

	case token.Return:
		return p.parseReturnStatement()

	case token.If:
		return p.parseIfStatement()

	case token.Ident:
		// 区分变量赋值（=）和普通表达式（如 ==）。
		if p.peekToken.Type == token.Assign {
			return p.parseAssignStatement()
		}
	}

	return p.parseExpressionStatement()
}

// parseExpression 使用 Pratt 优先级规则解析表达式。
func (p *Parser) parseExpression(prec precedence) (ast.Expression, error) {
	prefix, ok := p.prefixParseFns[p.curToken.Type]
	if !ok {
		return nil, fmt.Errorf(
			"no prefix parser for %s at %d:%d",
			p.curToken.Type,
			p.curToken.Start.Line,
			p.curToken.Start.Column,
		)
	}

	left, err := prefix()
	if err != nil {
		return nil, err
	}

	// 只有下一个运算符的优先级更高，才继续组合表达式。
	for prec < p.peekPrecedence() {
		infix, ok := p.infixParseFns[p.peekToken.Type]
		if !ok {
			break
		}

		p.nextToken()
		left, err = infix(left)
		if err != nil {
			return nil, err
		}
	}

	return left, nil
}

// parseIntegerLiteral 解析 int64 整数字面量。
func (p *Parser) parseIntegerLiteral() (ast.Expression, error) {
	if p.curToken.Type != token.Integer {
		return nil, fmt.Errorf("expected INTEGER, got %s", p.curToken.Type)
	}

	value, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer %q: %w", p.curToken.Literal, err)
	}

	return &ast.IntegerLiteral{
		Value: value,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseBooleanLiteral 解析 hai / iie 布尔字面量。
func (p *Parser) parseBooleanLiteral() (ast.Expression, error) {
	if p.curToken.Type != token.True && p.curToken.Type != token.False {
		return nil, fmt.Errorf("expected BOOLEAN, got %s", p.curToken.Type)
	}

	return &ast.BooleanLiteral{
		Value: p.curToken.Type == token.True,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseStringLiteral 保存 Lexer 已解码的字符串内容。
func (p *Parser) parseStringLiteral() (ast.Expression, error) {
	return &ast.StringLiteral{
		Value: p.curToken.Literal,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseNilLiteral 解析 mu 空值字面量。
func (p *Parser) parseNilLiteral() (ast.Expression, error) {
	return &ast.NilLiteral{
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseIdentifier 解析标识符。
func (p *Parser) parseIdentifier() (ast.Expression, error) {
	return &ast.Identifier{
		Name: p.curToken.Literal,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseGroupedExpression 解析括号分组，并保留括号的源码范围。
func (p *Parser) parseGroupedExpression() (ast.Expression, error) {
	open := p.curToken
	p.nextToken()

	// 括号内从最低优先级重新解析。
	expr, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.RParen {
		return nil, fmt.Errorf("expected ')', got %s", p.peekToken.Type)
	}
	p.nextToken()

	return &ast.GroupedExpression{
		Expr: expr,
		Ranges: ast.Span{
			Start: open.Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parsePrefixExpression 解析一元负号与逻辑非。
func (p *Parser) parsePrefixExpression() (ast.Expression, error) {
	operator := p.curToken
	p.nextToken()

	// 一元运算使用 precPrefix，避免吞掉后面的低优先级运算。
	right, err := p.parseExpression(precPrefix)
	if err != nil {
		return nil, err
	}

	return &ast.PrefixExpression{
		Operator: operator.Type,
		Right:    right,
		Range: ast.Span{
			Start: operator.Start,
			End:   right.Span().End,
		},
	}, nil
}

// parseInfixExpression 解析算术、比较和逻辑中缀表达式。
func (p *Parser) parseInfixExpression(left ast.Expression) (ast.Expression, error) {
	operator := p.curToken.Type
	prec := p.curPrecedence()
	p.nextToken()

	// 使用当前运算符优先级解析右侧，保持左结合性。
	right, err := p.parseExpression(prec)
	if err != nil {
		return nil, err
	}

	return &ast.InfixExpression{
		Left:     left,
		Operator: operator,
		Right:    right,
		Range: ast.Span{
			Start: left.Span().Start,
			End:   right.Span().End,
		},
	}, nil
}

// parseCallExpression 解析函数调用和逗号分隔的实参。
func (p *Parser) parseCallExpression(function ast.Expression) (ast.Expression, error) {
	call := &ast.CallExpression{
		Function:  function,
		Arguments: make([]ast.Expression, 0),
	}

	// 空实参列表，例如 chant()。
	if p.peekToken.Type == token.RParen {
		p.nextToken()
		call.Range = ast.Span{
			Start: function.Span().Start,
			End:   p.curToken.End,
		}
		return call, nil
	}

	p.nextToken()
	arg, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}
	call.Arguments = append(call.Arguments, arg)

	// 每遇到一个逗号，就继续解析下一个实参。
	for p.peekToken.Type == token.Comma {
		p.nextToken() // 到逗号
		p.nextToken() // 到下一个实参

		arg, err = p.parseExpression(precLowest)
		if err != nil {
			return nil, err
		}
		call.Arguments = append(call.Arguments, arg)
	}

	if p.peekToken.Type != token.RParen {
		return nil, fmt.Errorf("expected ')' after function arguments, got %s", p.peekToken.Type)
	}
	p.nextToken()

	call.Range = ast.Span{
		Start: function.Span().Start,
		End:   p.curToken.End,
	}
	return call, nil
}

// parseExpressionStatement 解析以分号结束的表达式语句。
func (p *Parser) parseExpressionStatement() (*ast.ExpressionStatement, error) {
	expr, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.Semicolon {
		return nil, fmt.Errorf("expected ';', got %s", p.peekToken.Type)
	}
	p.nextToken()

	return &ast.ExpressionStatement{
		Expr: expr,
		Range: ast.Span{
			Start: expr.Span().Start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseLetStatement 解析 kaku 变量声明语句。
func (p *Parser) parseLetStatement() (*ast.LetStatement, error) {
	start := p.curToken.Start

	if p.peekToken.Type != token.Ident {
		return nil, fmt.Errorf("expected identifier after kaku, got %s", p.peekToken.Type)
	}
	p.nextToken()

	name := &ast.Identifier{
		Name: p.curToken.Literal,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}

	if p.peekToken.Type != token.Assign {
		return nil, fmt.Errorf("expected '=' after %q, got %s", name.Name, p.peekToken.Type)
	}

	// 越过 =，到初始化表达式。
	p.nextToken()
	p.nextToken()

	value, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.Semicolon {
		return nil, fmt.Errorf("expected ';', got %s", p.peekToken.Type)
	}
	p.nextToken()

	return &ast.LetStatement{
		Name:  name,
		Value: value,
		Range: ast.Span{
			Start: start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseAssignStatement 解析已有变量的赋值语句。
func (p *Parser) parseAssignStatement() (*ast.AssignStatement, error) {
	start := p.curToken.Start

	name := &ast.Identifier{
		Name: p.curToken.Literal,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}

	if p.peekToken.Type != token.Assign {
		return nil, fmt.Errorf("expected '=' after %q, got %s", name.Name, p.peekToken.Type)
	}

	// 越过 =，到右侧表达式。
	p.nextToken()
	p.nextToken()

	value, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.Semicolon {
		return nil, fmt.Errorf("expected ';', got %s", p.peekToken.Type)
	}
	p.nextToken()

	return &ast.AssignStatement{
		Name:  name,
		Value: value,
		Range: ast.Span{
			Start: start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseBlockStatement 解析大括号中的多条语句。
func (p *Parser) parseBlockStatement() (*ast.BlockStatement, error) {
	open := p.curToken
	block := &ast.BlockStatement{
		Statements: make([]ast.Statement, 0),
	}

	p.nextToken()
	for p.curToken.Type != token.RBrace {
		if p.curToken.Type == token.EOF {
			return nil, fmt.Errorf(
				"expected '}' to close block at %d:%d",
				open.Start.Line,
				open.Start.Column,
			)
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, stmt)
		p.nextToken()
	}

	// 返回时停在当前代码块的右大括号。
	block.Range = ast.Span{
		Start: open.Start,
		End:   p.curToken.End,
	}
	return block, nil
}

// parseIfStatement 解析 moshi / sore 条件语句。
func (p *Parser) parseIfStatement() (*ast.IfStatement, error) {
	start := p.curToken.Start

	if p.peekToken.Type != token.LParen {
		return nil, fmt.Errorf("expected '(' after moshi, got %s", p.peekToken.Type)
	}

	// 越过左括号，进入条件表达式。
	p.nextToken()
	p.nextToken()

	condition, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.RParen {
		return nil, fmt.Errorf("expected ')' after condition, got %s", p.peekToken.Type)
	}
	p.nextToken()

	if p.peekToken.Type != token.LBrace {
		return nil, fmt.Errorf("expected '{' after moshi condition, got %s", p.peekToken.Type)
	}
	p.nextToken()

	consequence, err := p.parseBlockStatement()
	if err != nil {
		return nil, err
	}

	// sore 可以省略；它出现时必须跟随代码块。
	var alternative *ast.BlockStatement
	end := consequence.Span().End
	if p.peekToken.Type == token.Else {
		p.nextToken()
		if p.peekToken.Type != token.LBrace {
			return nil, fmt.Errorf("expected '{' after sore, got %s", p.peekToken.Type)
		}
		p.nextToken()

		alternative, err = p.parseBlockStatement()
		if err != nil {
			return nil, err
		}
		end = alternative.Span().End
	}

	return &ast.IfStatement{
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
		Range: ast.Span{
			Start: start,
			End:   end,
		},
	}, nil
}

// parseReturnStatement 解析 kaesu 返回语句。
func (p *Parser) parseReturnStatement() (*ast.ReturnStatement, error) {
	start := p.curToken.Start
	p.nextToken()

	value, err := p.parseExpression(precLowest)
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.Semicolon {
		return nil, fmt.Errorf("expected ';' after kaesu expression, got %s", p.peekToken.Type)
	}
	p.nextToken()

	return &ast.ReturnStatement{
		Value: value,
		Range: ast.Span{
			Start: start,
			End:   p.curToken.End,
		},
	}, nil
}

// parseFunctionParameters 解析 waza 定义中的形参列表。
func (p *Parser) parseFunctionParameters() ([]*ast.Identifier, error) {
	params := make([]*ast.Identifier, 0)

	// 空形参列表，例如 waza f() {}。
	if p.peekToken.Type == token.RParen {
		p.nextToken()
		return params, nil
	}

	for {
		p.nextToken()
		if p.curToken.Type != token.Ident {
			return nil, fmt.Errorf("expected parameter name, got %s", p.curToken.Type)
		}

		params = append(params, &ast.Identifier{
			Name: p.curToken.Literal,
			Range: ast.Span{
				Start: p.curToken.Start,
				End:   p.curToken.End,
			},
		})

		if p.peekToken.Type != token.Comma {
			break
		}
		p.nextToken()
	}

	if p.peekToken.Type != token.RParen {
		return nil, fmt.Errorf("expected ')' after parameters, got %s", p.peekToken.Type)
	}
	p.nextToken()

	return params, nil
}

// parseFunctionStatement 解析 waza 函数定义语句。
func (p *Parser) parseFunctionStatement() (*ast.FunctionStatement, error) {
	start := p.curToken.Start

	if p.peekToken.Type != token.Ident {
		return nil, fmt.Errorf("expected function name after waza, got %s", p.peekToken.Type)
	}
	p.nextToken()

	name := &ast.Identifier{
		Name: p.curToken.Literal,
		Range: ast.Span{
			Start: p.curToken.Start,
			End:   p.curToken.End,
		},
	}

	if p.peekToken.Type != token.LParen {
		return nil, fmt.Errorf("expected '(' after function name, got %s", p.peekToken.Type)
	}
	p.nextToken()

	params, err := p.parseFunctionParameters()
	if err != nil {
		return nil, err
	}

	if p.peekToken.Type != token.LBrace {
		return nil, fmt.Errorf("expected '{' after parameters, got %s", p.peekToken.Type)
	}
	p.nextToken()

	body, err := p.parseBlockStatement()
	if err != nil {
		return nil, err
	}

	return &ast.FunctionStatement{
		Name:       name,
		Parameters: params,
		Body:       body,
		Range: ast.Span{
			Start: start,
			End:   body.Span().End,
		},
	}, nil
}
