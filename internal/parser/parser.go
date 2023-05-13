// Package parser turns a token stream into a surface AST.
//
// The grammar is the brace dialect of a Scala 3 kernel. Newlines may separate
// statements. A newline does not end a statement before an operator, '.', '(',
// or '['. Indented bodies are accepted after '=', then, else, do, yield, with,
// and a trailing ':' on a template.
package parser

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/diag"
	"github.com/Aneesh495/fathom/internal/scanner"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

// Parse scans and parses one file. Scan errors and parse errors share a list.
func Parse(file *span.File) (*ast.File, *diag.List) {
	toks, scanDiags := scanner.Scan(file)
	p := &Parser{
		file:  file,
		toks:  toks,
		diags: diag.NewList(80),
	}
	if scanDiags != nil {
		for _, d := range scanDiags.Items {
			p.diags.Add(d)
		}
	}
	if len(p.toks) == 0 {
		p.toks = []token.Token{{Kind: token.EOF}}
	}
	return p.parseFile(), p.diags
}

// Parser is a recursive descent parser over a buffered token stream.
type Parser struct {
	file     *span.File
	toks     []token.Token
	i        int
	prev     token.Token
	diags    *diag.List
	noRefine int
}

func (p *Parser) tok() token.Token {
	if p.i >= len(p.toks) {
		return token.Token{Kind: token.EOF, Span: span.Span{File: p.file}}
	}
	return p.toks[p.i]
}

func (p *Parser) kind() token.Kind { return p.tok().Kind }

func (p *Parser) lit() string { return p.tok().Lit }

func (p *Parser) span() span.Span { return p.tok().Span }

func (p *Parser) next() token.Token {
	t := p.tok()
	p.prev = t
	if t.Kind != token.EOF && p.i+1 < len(p.toks) {
		p.i++
	}
	return t
}

func (p *Parser) peekKind() token.Kind {
	if p.i+1 >= len(p.toks) {
		return token.EOF
	}
	return p.toks[p.i+1].Kind
}

func (p *Parser) eat(k token.Kind) bool {
	if p.kind() == k {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expect(k token.Kind) bool {
	if p.eat(k) {
		return true
	}
	p.err("expected " + k.String())
	return false
}

func (p *Parser) skipSemis() {
	for p.kind() == token.Semi {
		p.next()
	}
}

func (p *Parser) err(msg string) { p.errAt(p.span(), msg) }

func (p *Parser) errAt(sp span.Span, msg string) {
	if p.diags == nil {
		p.diags = diag.NewList(0)
	}
	p.diags.Errorf(sp, "%s", msg)
}

func (p *Parser) col() int { return p.span().Position().Col }

func (p *Parser) line() int { return p.span().Position().Line }

func (p *Parser) badExpr() ast.Expr {
	return &ast.Ident{S: p.span(), Name: "_error"}
}

func (p *Parser) parseIdent() *ast.Ident {
	sp := p.span()
	if p.kind() != token.Ident {
		p.err("expected identifier")
		return &ast.Ident{S: sp, Name: "_error"}
	}
	quoted := false
	if sp.File != nil {
		src := sp.File.Src
		if sp.Start >= 0 && sp.Start < len(src) && src[sp.Start] == '`' {
			quoted = true
		}
	}
	name := p.lit()
	p.next()
	return &ast.Ident{S: sp, Name: name, Quoted: quoted}
}

func (p *Parser) parseFile() *ast.File {
	f := &ast.File{S: p.span()}
	p.skipSemis()
	if p.eat(token.Package) {
		f.Package = p.parseQualName()
		p.skipSemis()
	}
	for p.kind() == token.Import {
		if im, ok := p.parseImport().(*ast.Import); ok {
			f.Imports = append(f.Imports, im)
		}
		p.skipSemis()
	}
	for p.kind() != token.EOF {
		before := p.i
		st := p.parseStmt()
		if st != nil {
			f.Stats = append(f.Stats, st)
		}
		p.skipSemis()
		if p.i == before {
			p.err("unexpected token")
			if p.kind() != token.EOF {
				p.next()
			} else {
				break
			}
		}
	}
	if len(p.toks) > 0 {
		last := p.toks[len(p.toks)-1].Span
		f.S = span.Merge(f.S, last)
	}
	return f
}

func (p *Parser) parseQualName() *ast.QualName {
	sp := p.span()
	id := p.parseIdent()
	q := &ast.QualName{S: id.S, Parts: []string{id.Name}}
	for p.eat(token.Dot) {
		id = p.parseIdent()
		q.Parts = append(q.Parts, id.Name)
		q.S = span.Merge(sp, id.S)
	}
	return q
}

func (p *Parser) parseImport() ast.Stmt {
	sp := p.span()
	p.expect(token.Import)
	path := &ast.QualName{S: p.span()}
	id := p.parseIdent()
	path.Parts = append(path.Parts, id.Name)
	path.S = id.S
	im := &ast.Import{S: sp, Path: path}
	for p.kind() == token.Dot {
		// Look through the dot so `._` and `.{` stay attached to this import.
		if p.peekKind() == token.LBrace || (p.peekKind() == token.Ident && p.toks[p.i+1].Lit == "_") {
			p.next() // dot
			if p.kind() == token.LBrace {
				p.next()
				p.skipSemis()
				for p.kind() != token.RBrace && p.kind() != token.EOF {
					if p.kind() == token.Ident && p.lit() == "_" {
						im.Wildcard = true
						p.next()
					} else {
						sel := ast.ImportSel{Span: p.span(), Name: p.parseIdent().Name}
						if p.eat(token.Arrow) {
							sel.Rename = p.parseIdent().Name
						}
						im.Selectors = append(im.Selectors, sel)
					}
					if !p.eat(token.Comma) {
						break
					}
					p.skipSemis()
				}
				p.expect(token.RBrace)
			} else {
				p.next() // _
				im.Wildcard = true
			}
			break
		}
		p.next()
		id = p.parseIdent()
		path.Parts = append(path.Parts, id.Name)
		path.S = span.Merge(path.S, id.S)
	}
	im.S = span.Merge(sp, p.prev.Span)
	return im
}

func (p *Parser) parseStmt() ast.Stmt {
	return p.parseStmtMode(false)
}

func (p *Parser) parseStmtMode(inEnum bool) ast.Stmt {
	before := p.i
	st := p.parseStmtBody(inEnum)
	if p.i == before && p.kind() != token.EOF {
		p.err("expected statement")
		p.next()
	}
	return st
}

func (p *Parser) parseStmtBody(inEnum bool) ast.Stmt {
	if inEnum && p.isEnumCase() {
		// Enum cases are collected by the template parser. If one shows up
		// here, skip it with a diagnostic rather than looping.
		p.err("enum case is only legal directly inside an enum")
		p.parseEnumCases()
		return nil
	}
	anns := p.parseAnns()
	mods := p.parseMods()
	switch p.kind() {
	case token.Class, token.Trait, token.Object, token.Enum:
		return p.parseClass(anns, mods)
	case token.Def:
		return p.parseDef(anns, mods)
	case token.Val, token.Var:
		return p.parseVal(anns, mods)
	case token.Type:
		return p.parseTypeDecl(anns, mods)
	case token.Given:
		return p.parseGiven(anns, mods)
	case token.Extension:
		return p.parseExt(anns, mods)
	case token.Import:
		if len(anns) > 0 || len(mods) > 0 {
			p.err("import cannot have modifiers")
		}
		return p.parseImport()
	}
	if len(anns) > 0 || len(mods) > 0 {
		p.err("modifier is not attached to a declaration")
	}
	e := p.parseExpr()
	if e == nil {
		return nil
	}
	return &ast.ExprStmt{S: e.Span(), X: e}
}

func (p *Parser) isEnumCase() bool {
	if p.kind() != token.Case {
		return false
	}
	pk := p.peekKind()
	return pk != token.Class && pk != token.Object
}

func (p *Parser) parseAnns() []ast.Annotation {
	var anns []ast.Annotation
	for p.kind() == token.At {
		sp := p.span()
		p.next()
		name := "_error"
		if p.kind() == token.Ident {
			name = p.lit()
			p.next()
		} else {
			p.err("expected annotation name")
		}
		var args []ast.Expr
		if p.eat(token.LParen) {
			if p.kind() != token.RParen {
				args = p.parseArgList()
			}
			p.expect(token.RParen)
		}
		anns = append(anns, ast.Annotation{Span: span.Merge(sp, p.prev.Span), Name: name, Args: args})
	}
	return anns
}

func (p *Parser) parseMods() []ast.Mod {
	var mods []ast.Mod
	for {
		k, ok := modKind(p.kind())
		if !ok {
			return mods
		}
		mods = append(mods, ast.Mod{Kind: k, Span: p.span()})
		p.next()
	}
}

func modKind(k token.Kind) (ast.ModKind, bool) {
	switch k {
	case token.Abstract:
		return ast.ModAbstract, true
	case token.Final:
		return ast.ModFinal, true
	case token.Sealed:
		return ast.ModSealed, true
	case token.Override:
		return ast.ModOverride, true
	case token.Lazy:
		return ast.ModLazy, true
	case token.Opaque:
		return ast.ModOpaque, true
	case token.Open:
		return ast.ModOpen, true
	case token.Infix:
		return ast.ModInfix, true
	case token.Inline:
		return ast.ModInline, true
	case token.Transparent:
		return ast.ModTransparent, true
	case token.Private:
		return ast.ModPrivate, true
	case token.Protected:
		return ast.ModProtected, true
	case token.Case:
		return ast.ModCase, true
	case token.Implicit:
		return ast.ModImplicit, true
	default:
		return 0, false
	}
}
