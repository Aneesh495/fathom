package parser

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

func (p *Parser) parseClass(anns []ast.Annotation, mods []ast.Mod) *ast.ClassDecl {
	sp := p.span()
	headerCol := p.col()
	kind := ast.KindClass
	switch p.kind() {
	case token.Trait:
		kind = ast.KindTrait
	case token.Object:
		kind = ast.KindObject
	case token.Enum:
		kind = ast.KindEnum
	case token.Class:
		kind = ast.KindClass
	default:
		p.err("expected class, trait, object, or enum")
		return &ast.ClassDecl{S: sp, Name: "_error", Kind: kind, Mods: mods, Anns: anns}
	}
	p.next()
	name := p.parseIdent()
	d := &ast.ClassDecl{
		S: sp, Mods: mods, Anns: anns, Kind: kind, Name: name.Name,
	}
	if kind != ast.KindObject && p.kind() == token.LBrack {
		d.TParams = p.parseTypeParams()
	}
	if p.kind() == token.LParen {
		d.HasParams = true
		p.next()
		if p.kind() != token.RParen {
			d.Params = p.parseParams()
		}
		p.expect(token.RParen)
	}
	if p.eat(token.Extends) {
		d.Parents = append(d.Parents, p.parseParent())
		for p.eat(token.With) {
			d.Parents = append(d.Parents, p.parseParent())
		}
	}
	body, cases := p.parseMaybeTemplate(kind == ast.KindEnum, headerCol)
	d.Body = body
	d.Cases = cases
	d.S = span.Merge(sp, p.prev.Span)
	return d
}

func (p *Parser) parseMaybeTemplate(inEnum bool, headerCol int) (*ast.Template, []*ast.EnumCase) {
	switch p.kind() {
	case token.LBrace:
		return p.parseBraceTemplate(inEnum)
	case token.Colon:
		p.next()
		return p.parseIndentedTemplate(inEnum, headerCol)
	default:
		return &ast.Template{S: p.span()}, nil
	}
}

func (p *Parser) parseBraceTemplate(inEnum bool) (*ast.Template, []*ast.EnumCase) {
	sp := p.span()
	p.expect(token.LBrace)
	t := &ast.Template{S: sp}
	cases := p.fillTemplate(t, inEnum, 0)
	end := p.span()
	p.expect(token.RBrace)
	t.S = span.Merge(sp, end)
	return t, cases
}

func (p *Parser) parseIndentedTemplate(inEnum bool, headerCol int) (*ast.Template, []*ast.EnumCase) {
	p.skipSemis()
	t := &ast.Template{S: p.span(), Indented: true}
	if p.kind() == token.EOF || p.kind() == token.End || p.col() <= headerCol {
		p.consumeEnd(headerCol)
		return t, nil
	}
	cases := p.fillTemplate(t, inEnum, p.col())
	p.consumeEnd(headerCol)
	if len(t.Stats) > 0 {
		t.S = t.Stats[0].Span()
	}
	return t, cases
}

func (p *Parser) fillTemplate(t *ast.Template, inEnum bool, bodyCol int) []*ast.EnumCase {
	p.skipSemis()
	if p.looksLikeSelfType() {
		id := p.parseIdent()
		t.SelfName = id.Name
		if p.eat(token.Colon) {
			t.SelfType = p.parseType()
		}
		p.expect(token.Arrow)
		p.skipSemis()
	}
	var cases []*ast.EnumCase
	for {
		if bodyCol > 0 && (p.kind() == token.EOF || p.kind() == token.End || p.col() < bodyCol) {
			break
		}
		if bodyCol == 0 && (p.kind() == token.RBrace || p.kind() == token.EOF) {
			break
		}
		if inEnum && p.isEnumCase() {
			cases = append(cases, p.parseEnumCases()...)
			p.skipSemis()
			continue
		}
		before := p.i
		st := p.parseStmt()
		if st != nil {
			t.Stats = append(t.Stats, st)
		}
		p.skipSemis()
		if p.i == before {
			break
		}
	}
	return cases
}

func (p *Parser) consumeEnd(headerCol int) {
	if p.kind() == token.End && (headerCol == 0 || p.col() <= headerCol+1 || p.col() < p.col()) {
		p.next()
		if p.kind() == token.Ident {
			p.next()
		}
	}
}

func (p *Parser) looksLikeSelfType() bool {
	if p.kind() != token.Ident {
		return false
	}
	if p.peekKind() == token.Arrow {
		return true
	}
	if p.peekKind() != token.Colon {
		return false
	}
	depth := 0
	for j := p.i + 2; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LParen, token.LBrack, token.LBrace:
			depth++
		case token.RParen, token.RBrack, token.RBrace:
			if depth == 0 {
				return false
			}
			depth--
		case token.Arrow:
			if depth == 0 {
				return true
			}
		case token.Semi, token.EOF:
			if depth == 0 {
				return false
			}
		}
	}
	return false
}

func (p *Parser) parseEnumCases() []*ast.EnumCase {
	p.expect(token.Case)
	var cases []*ast.EnumCase
	for {
		name := p.parseIdent()
		c := &ast.EnumCase{S: name.S, Name: name.Name}
		if p.eat(token.LParen) {
			c.HasParams = true
			if p.kind() != token.RParen {
				c.Params = p.parseParams()
			}
			end := p.span()
			p.expect(token.RParen)
			c.S = span.Merge(name.S, end)
		}
		if p.kind() == token.LBrace || p.kind() == token.Colon {
			body, _ := p.parseMaybeTemplate(false, p.col())
			c.Body = body
		}
		cases = append(cases, c)
		if !p.eat(token.Comma) {
			break
		}
	}
	return cases
}

func (p *Parser) parseDef(anns []ast.Annotation, mods []ast.Mod) *ast.DefDecl {
	sp := p.span()
	headerCol := p.col()
	p.expect(token.Def)
	name, nameSpan := p.parseOpOrIdent()
	d := &ast.DefDecl{S: nameSpan, Mods: mods, Anns: anns, Name: name}
	if p.kind() == token.LBrack {
		d.TParams = p.parseTypeParams()
	}
	for p.kind() == token.LParen {
		d.Lists = append(d.Lists, p.parseParamList())
	}
	if p.eat(token.Colon) {
		d.Result = p.parseType()
	}
	if p.kind() == token.Assign {
		eqLine := p.line()
		p.next()
		d.Body = p.parseBodyAfter(headerCol, eqLine, token.EOF)
		d.S = span.Merge(sp, d.Body.Span())
	} else {
		d.S = span.Merge(sp, p.prev.Span)
	}
	return d
}

func (p *Parser) parseOpOrIdent() (string, span.Span) {
	switch p.kind() {
	case token.Ident, token.Op:
		name := p.lit()
		sp := p.span()
		p.next()
		return name, sp
	default:
		p.err("expected name")
		return "_error", p.span()
	}
}

func (p *Parser) parseVal(anns []ast.Annotation, mods []ast.Mod) *ast.ValDecl {
	sp := p.span()
	headerCol := p.col()
	mutable := p.kind() == token.Var
	p.next()
	d := &ast.ValDecl{S: sp, Mods: mods, Mutable: mutable}
	if p.simpleBinder() {
		id := p.parseIdent()
		d.Name = id.Name
		d.S = id.S
	} else {
		d.Pat = p.parsePattern()
		d.S = d.Pat.Span()
	}
	if p.eat(token.Colon) {
		d.Type = p.parseType()
	}
	if p.kind() == token.Assign {
		eqLine := p.line()
		p.next()
		d.Body = p.parseBodyAfter(headerCol, eqLine, token.EOF)
		d.S = span.Merge(sp, d.Body.Span())
	}
	_ = anns
	return d
}

func (p *Parser) simpleBinder() bool {
	if p.kind() != token.Ident {
		return false
	}
	switch p.peekKind() {
	case token.Colon, token.Assign, token.Semi, token.EOF, token.RBrace, token.End:
		return true
	default:
		return false
	}
}

func (p *Parser) parseTypeDecl(anns []ast.Annotation, mods []ast.Mod) *ast.TypeDecl {
	sp := p.span()
	p.expect(token.Type)
	name := p.parseIdent()
	d := &ast.TypeDecl{S: span.Merge(sp, name.S), Mods: mods, Name: name.Name}
	if p.kind() == token.LBrack {
		d.TParams = p.parseTypeParams()
	}
	if p.eat(token.Assign) {
		d.Body = p.parseType()
		d.S = span.Merge(d.S, d.Body.Span())
		_ = anns
		return d
	}
	for {
		if p.eat(token.Supertype) {
			d.Lo = p.parseType()
			continue
		}
		if p.eat(token.Subtype) {
			d.Hi = p.parseType()
			continue
		}
		break
	}
	if d.Hi != nil {
		d.S = span.Merge(d.S, d.Hi.Span())
	}
	_ = anns
	return d
}

func (p *Parser) parseGiven(anns []ast.Annotation, mods []ast.Mod) *ast.GivenDecl {
	sp := p.span()
	headerCol := p.col()
	p.expect(token.Given)
	g := &ast.GivenDecl{S: sp, Mods: mods}
	switch {
	case p.kind() == token.LBrack:
		g.TParams = p.parseTypeParams()
		for p.kind() == token.LParen {
			g.Lists = append(g.Lists, p.parseParamList())
		}
		p.expect(token.Colon)
		g.Type = p.parseType()
	case p.kind() == token.Ident && p.givenIsNamed():
		g.Name = p.lit()
		p.next()
		if p.kind() == token.LBrack {
			g.TParams = p.parseTypeParams()
		}
		for p.kind() == token.LParen {
			g.Lists = append(g.Lists, p.parseParamList())
		}
		p.expect(token.Colon)
		g.Type = p.parseType()
	default:
		g.Type = p.parseType()
	}
	if p.kind() == token.Assign {
		eqLine := p.line()
		p.next()
		g.Body = p.parseBodyAfter(headerCol, eqLine, token.EOF)
		g.S = span.Merge(sp, g.Body.Span())
	} else if p.eat(token.With) {
		g.BodyTemplate = p.parseWithBody(headerCol)
		if g.BodyTemplate != nil {
			g.S = span.Merge(sp, g.BodyTemplate.S)
		}
	} else {
		p.err("expected '=' or 'with' after given")
	}
	_ = anns
	return g
}

func (p *Parser) givenIsNamed() bool {
	if p.peekKind() == token.Colon {
		return true
	}
	if p.peekKind() != token.LBrack && p.peekKind() != token.LParen {
		return false
	}
	depth := 0
	for j := p.i + 1; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LParen, token.LBrack, token.LBrace:
			depth++
		case token.RParen, token.RBrack, token.RBrace:
			depth--
		case token.Colon:
			if depth == 0 {
				return true
			}
		case token.With, token.Assign, token.Semi, token.EOF:
			if depth == 0 {
				return false
			}
		}
	}
	return false
}

func (p *Parser) parseWithBody(headerCol int) *ast.Template {
	if p.kind() == token.LBrace || p.kind() == token.Colon {
		t, _ := p.parseMaybeTemplate(false, headerCol)
		return t
	}
	t, _ := p.parseIndentedTemplate(false, headerCol)
	return t
}

func (p *Parser) parseExt(anns []ast.Annotation, mods []ast.Mod) *ast.ExtDecl {
	sp := p.span()
	headerCol := p.col()
	p.expect(token.Extension)
	d := &ast.ExtDecl{S: sp}
	if p.kind() == token.LBrack {
		d.TParams = p.parseTypeParams()
	}
	if p.kind() == token.LParen {
		list := p.parseParamList()
		if len(list.Params) != 1 {
			p.errAt(list.S, "extension clause expects exactly one parameter")
		}
		if len(list.Params) > 0 {
			d.Param = list.Params[0]
		}
	} else {
		p.err("extension expects a parameter list")
	}
	if p.kind() == token.LBrace || p.kind() == token.Colon {
		d.Body, _ = p.parseMaybeTemplate(false, headerCol)
	} else {
		d.Body, _ = p.parseIndentedTemplate(false, headerCol)
	}
	if d.Body != nil {
		d.S = span.Merge(sp, d.Body.S)
	}
	if len(mods) > 0 || len(anns) > 0 {
		p.errAt(sp, "extension does not take modifiers")
	}
	return d
}

func (p *Parser) parseParamList() *ast.ParamList {
	sp := p.span()
	p.expect(token.LParen)
	list := &ast.ParamList{S: sp}
	if p.eat(token.Using) {
		list.Using = true
	}
	if p.kind() != token.RParen {
		list.Params = p.parseParams()
	}
	end := p.span()
	p.expect(token.RParen)
	list.S = span.Merge(sp, end)
	return list
}

func (p *Parser) parseParams() []*ast.Param {
	var ps []*ast.Param
	for {
		p.skipSemis()
		if p.kind() == token.RParen || p.kind() == token.EOF {
			break
		}
		ps = append(ps, p.parseParam())
		p.skipSemis()
		if !p.eat(token.Comma) {
			break
		}
	}
	return ps
}

func (p *Parser) parseParam() *ast.Param {
	sp := p.span()
	isVal := false
	isVar := false
	if p.eat(token.Val) {
		isVal = true
	} else if p.eat(token.Var) {
		isVar = true
	}
	name, nameSpan := p.parseOpOrIdent()
	param := &ast.Param{S: span.Merge(sp, nameSpan), Name: name, IsVal: isVal, IsVar: isVar}
	if p.eat(token.Colon) {
		param.Type = p.parseType()
		param.S = span.Merge(param.S, param.Type.Span())
	} else {
		p.err("expected parameter type")
	}
	return param
}

func (p *Parser) parseTypeParams() []*ast.TypeParam {
	p.expect(token.LBrack)
	var ps []*ast.TypeParam
	if p.kind() != token.RBrack {
		for {
			p.skipSemis()
			ps = append(ps, p.parseTypeParam())
			p.skipSemis()
			if !p.eat(token.Comma) {
				break
			}
		}
	}
	p.expect(token.RBrack)
	return ps
}

func (p *Parser) parseTypeParam() *ast.TypeParam {
	sp := p.span()
	variance := ast.Invariant
	if p.kind() == token.Op && p.lit() == "+" {
		variance = ast.Covariant
		p.next()
	} else if p.kind() == token.Op && p.lit() == "-" {
		variance = ast.Contravariant
		p.next()
	}
	name := "_"
	if p.kind() == token.Ident {
		name = p.lit()
		p.next()
	} else if p.kind() == token.Question {
		p.next()
	} else {
		p.err("expected type parameter")
	}
	tp := &ast.TypeParam{S: sp, Name: name, Variance: variance}
	if p.eat(token.LBrack) {
		if p.kind() != token.RBrack {
			for {
				tp.Params = append(tp.Params, p.parseTypeParam())
				if !p.eat(token.Comma) {
					break
				}
			}
		}
		p.expect(token.RBrack)
	}
	for {
		if p.eat(token.Supertype) {
			tp.Lo = p.parseTypeAtom()
			continue
		}
		if p.eat(token.Subtype) {
			tp.Hi = p.parseTypeAtom()
			continue
		}
		break
	}
	return tp
}
