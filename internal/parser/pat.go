package parser

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

func (p *Parser) parsePattern() ast.Pattern {
	left := p.parsePatTyped()
	if p.kind() == token.Op && p.lit() == "|" {
		alts := []ast.Pattern{left}
		for p.kind() == token.Op && p.lit() == "|" {
			p.next()
			alts = append(alts, p.parsePatTyped())
		}
		return &ast.AltPat{S: span.Merge(left.Span(), alts[len(alts)-1].Span()), Pats: alts}
	}
	return left
}

func (p *Parser) parsePatTyped() ast.Pattern {
	pat := p.parsePatAtom()
	if p.eat(token.Colon) {
		typ := p.parseType()
		pat = &ast.TypedPat{S: span.Merge(pat.Span(), typ.Span()), Pat: pat, Type: typ}
	}
	if b, ok := pat.(*ast.BindPat); ok && b.Pat == nil && p.eat(token.At) {
		inner := p.parsePatTyped()
		return &ast.BindPat{S: span.Merge(b.S, inner.Span()), Name: b.Name, Pat: inner}
	}
	return pat
}

func (p *Parser) parsePatAtom() ast.Pattern {
	sp := p.span()
	switch p.kind() {
	case token.Int, token.Float, token.String, token.Char, token.True, token.False, token.Null:
		lit, _ := p.parseLit().(*ast.LiteralExpr)
		return &ast.LitPat{S: sp, Lit: lit}
	case token.LParen:
		return p.parsePatParen()
	case token.Ident, token.This, token.Super:
		e := p.parsePatPath()
		if p.kind() == token.LParen {
			p.next()
			var pats []ast.Pattern
			if p.kind() != token.RParen {
				for {
					pats = append(pats, p.parsePattern())
					if !p.eat(token.Comma) {
						break
					}
				}
			}
			end := p.span()
			p.expect(token.RParen)
			return &ast.CtorPat{S: span.Merge(sp, end), Ctor: e, Pats: pats, HasArgs: true}
		}
		if id, ok := e.(*ast.Ident); ok {
			if id.Name == "_" && !id.Quoted {
				return &ast.WildPat{S: id.S}
			}
			if ast.IsVarName(id.Name, id.Quoted) {
				return &ast.BindPat{S: id.S, Name: id.Name}
			}
		}
		return &ast.CtorPat{S: e.Span(), Ctor: e, HasArgs: false}
	default:
		p.err("expected pattern")
		if p.kind() != token.EOF {
			p.next()
		}
		return &ast.WildPat{S: sp}
	}
}

func (p *Parser) parsePatParen() ast.Pattern {
	sp := p.span()
	p.expect(token.LParen)
	p.skipSemis()
	if p.eat(token.RParen) {
		return &ast.TuplePat{S: span.Merge(sp, p.prev.Span)}
	}
	first := p.parsePattern()
	p.skipSemis()
	if p.eat(token.Comma) {
		pats := []ast.Pattern{first}
		for {
			p.skipSemis()
			if p.kind() == token.RParen || p.kind() == token.EOF {
				break
			}
			pats = append(pats, p.parsePattern())
			p.skipSemis()
			if !p.eat(token.Comma) {
				break
			}
		}
		end := p.span()
		p.expect(token.RParen)
		return &ast.TuplePat{S: span.Merge(sp, end), Pats: pats}
	}
	end := p.span()
	p.expect(token.RParen)
	_ = end
	return first
}

func (p *Parser) parsePatPath() ast.Expr {
	var e ast.Expr
	switch p.kind() {
	case token.This:
		e = &ast.ThisExpr{S: p.span()}
		p.next()
	case token.Super:
		e = p.parseSuper()
	default:
		e = p.parseIdent()
	}
	for p.kind() == token.Dot && p.peekKind() == token.Ident {
		p.next()
		id := p.parseIdent()
		e = &ast.SelectExpr{S: span.Merge(e.Span(), id.S), X: e, Name: id.Name}
	}
	return e
}
