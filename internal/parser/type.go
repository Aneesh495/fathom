package parser

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

func (p *Parser) parseType() ast.TypeNode {
	if p.kind() == token.Arrow {
		sp := p.span()
		p.next()
		inner := p.parseType()
		return &ast.ByNameType{S: span.Merge(sp, inner.Span()), X: inner}
	}
	left := p.parseUnionType()
	if p.eat(token.Arrow) {
		params := funParams(left)
		res := p.parseType()
		return &ast.FunType{S: span.Merge(left.Span(), res.Span()), Params: params, Result: res}
	}
	return left
}

func funParams(t ast.TypeNode) []ast.TypeNode {
	if tup, ok := t.(*ast.TupleType); ok {
		return tup.Elems
	}
	return []ast.TypeNode{t}
}

func (p *Parser) parseUnionType() ast.TypeNode {
	left := p.parseInterType()
	for p.kind() == token.Op && p.lit() == "|" {
		p.next()
		right := p.parseInterType()
		left = &ast.InfixType{S: span.Merge(left.Span(), right.Span()), Op: "|", Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseInterType() ast.TypeNode {
	left := p.parseTypeApp()
	for p.kind() == token.Op && p.lit() == "&" {
		p.next()
		right := p.parseTypeApp()
		left = &ast.InfixType{S: span.Merge(left.Span(), right.Span()), Op: "&", Left: left, Right: right}
	}
	return left
}

// parseTypeAtom is a type that stops before ':' so bounds like `A <: B` do not
// swallow a following parameter separator. It still accepts applications.
func (p *Parser) parseTypeAtom() ast.TypeNode {
	return p.parseTypeApp()
}

func (p *Parser) parseTypeApp() ast.TypeNode {
	t := p.parseSimpleType()
	for {
		switch p.kind() {
		case token.LBrack:
			args := p.parseTypeArgs()
			end := t.Span()
			if len(args) > 0 {
				end = args[len(args)-1].Span()
			}
			t = &ast.ApplyType{S: span.Merge(t.Span(), end), Fun: t, Args: args}
		case token.Hash:
			p.next()
			name := p.parseIdent()
			t = &ast.ProjectType{S: span.Merge(t.Span(), name.S), X: t, Name: name.Name}
		case token.Dot:
			p.next()
			if p.eat(token.Type) {
				path := typeToPath(t)
				if path == nil {
					p.err("singleton type expects a term path")
					path = p.badExpr()
				}
				t = &ast.SingletonType{S: span.Merge(t.Span(), p.prev.Span), Path: path}
				return t
			}
			if p.kind() == token.This {
				// pkg.this.A is unusual in type position. Record a path type.
				th := &ast.ThisExpr{S: p.span()}
				p.next()
				if p.eat(token.Dot) {
					if p.eat(token.Type) {
						return &ast.SingletonType{S: span.Merge(t.Span(), p.prev.Span), Path: th}
					}
					name := p.parseIdent()
					return &ast.PathType{S: span.Merge(t.Span(), name.S), Prefix: th, Name: name.Name}
				}
				return &ast.SingletonType{S: th.S, Path: th}
			}
			name := p.parseIdent()
			t = &ast.SelectType{S: span.Merge(t.Span(), name.S), X: t, Name: name.Name}
		case token.LBrace:
			if p.noRefine > 0 {
				return t
			}
			body, _ := p.parseBraceTemplate(false)
			t = &ast.RefineType{S: span.Merge(t.Span(), body.S), X: t, Stats: body.Stats}
		default:
			return t
		}
	}
}

func (p *Parser) parseTypeArgs() []ast.TypeNode {
	p.expect(token.LBrack)
	var args []ast.TypeNode
	if p.kind() != token.RBrack {
		for {
			p.skipSemis()
			args = append(args, p.parseTypeArg())
			p.skipSemis()
			if !p.eat(token.Comma) {
				break
			}
		}
	}
	p.expect(token.RBrack)
	return args
}

func (p *Parser) parseTypeArg() ast.TypeNode {
	if p.kind() == token.Question || (p.kind() == token.Ident && p.lit() == "_") {
		sp := p.span()
		p.next()
		w := &ast.WildType{S: sp}
		if p.eat(token.Supertype) {
			w.Lo = p.parseTypeAtom()
		}
		if p.eat(token.Subtype) {
			w.Hi = p.parseTypeAtom()
		}
		return w
	}
	return p.parseType()
}

func (p *Parser) parseSimpleType() ast.TypeNode {
	sp := p.span()
	switch p.kind() {
	case token.Ident:
		if p.lit() == "_" {
			p.next()
			return &ast.WildType{S: sp}
		}
		id := p.parseIdent()
		return &ast.IdentType{S: id.S, Name: id.Name}
	case token.This:
		th := &ast.ThisExpr{S: sp}
		p.next()
		if p.eat(token.Dot) {
			if p.eat(token.Type) {
				return &ast.SingletonType{S: span.Merge(sp, p.prev.Span), Path: th}
			}
			name := p.parseIdent()
			return &ast.PathType{S: span.Merge(sp, name.S), Prefix: th, Name: name.Name}
		}
		return &ast.SingletonType{S: th.S, Path: th}
	case token.LParen:
		return p.parseParenType()
	case token.Int, token.Float, token.String, token.Char, token.True, token.False, token.Null:
		lit, _ := p.parseLit().(*ast.LiteralExpr)
		return &ast.LitType{S: sp, Lit: lit}
	case token.Question:
		p.next()
		return &ast.WildType{S: sp}
	default:
		p.err("expected type")
		return &ast.IdentType{S: sp, Name: "_error"}
	}
}

func (p *Parser) parseParenType() ast.TypeNode {
	sp := p.span()
	p.expect(token.LParen)
	p.skipSemis()
	saved := p.noRefine
	p.noRefine = 0
	defer func() { p.noRefine = saved }()
	if p.eat(token.RParen) {
		return &ast.TupleType{S: span.Merge(sp, p.prev.Span)}
	}
	first := p.parseType()
	p.skipSemis()
	if p.eat(token.Comma) {
		elems := []ast.TypeNode{first}
		for {
			p.skipSemis()
			if p.kind() == token.RParen || p.kind() == token.EOF {
				break
			}
			elems = append(elems, p.parseType())
			p.skipSemis()
			if !p.eat(token.Comma) {
				break
			}
		}
		end := p.span()
		p.expect(token.RParen)
		return &ast.TupleType{S: span.Merge(sp, end), Elems: elems}
	}
	end := p.span()
	p.expect(token.RParen)
	// A single parenthesized type keeps its own identity. The parentheses are
	// only grouping. Tuple types require a comma.
	_ = end
	return first
}

func typeToPath(t ast.TypeNode) ast.Expr {
	switch n := t.(type) {
	case *ast.IdentType:
		return &ast.Ident{S: n.S, Name: n.Name}
	case *ast.SelectType:
		x := typeToPath(n.X)
		if x == nil {
			return nil
		}
		return &ast.SelectExpr{S: n.S, X: x, Name: n.Name}
	case *ast.SingletonType:
		return n.Path
	case *ast.PathType:
		return &ast.SelectExpr{S: n.S, X: n.Prefix, Name: n.Name}
	default:
		return nil
	}
}
