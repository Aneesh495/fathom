package parser

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

func (p *Parser) parseExpr() ast.Expr {
	e := p.parseAssign()
	for p.eat(token.Match) {
		cases := p.parseCaseBlock()
		end := e.Span()
		if n := len(cases); n > 0 {
			end = cases[n-1].Span()
		}
		e = &ast.MatchExpr{S: span.Merge(e.Span(), end), Scrut: e, Cases: cases}
	}
	return e
}

func (p *Parser) parseAssign() ast.Expr {
	left := p.parseBin(token.PrecLetter)
	if left == nil {
		return p.badExpr()
	}
	if p.kind() == token.Assign || (p.kind() == token.Op && token.IsAssignmentOp(p.lit())) {
		op := "="
		if p.kind() == token.Op {
			op = p.lit()
		}
		p.next()
		right := p.parseExpr()
		return &ast.AssignExpr{S: span.Merge(left.Span(), right.Span()), LHS: left, Op: op, RHS: right}
	}
	return left
}

func (p *Parser) parseBin(min token.Prec) ast.Expr {
	left := p.parseUnary()
	if left == nil {
		return p.badExpr()
	}
	for {
		if p.kind() == token.Colon {
			if token.PrecCons < min {
				break
			}
			p.next()
			typ := p.parseType()
			left = &ast.AscribeExpr{S: span.Merge(left.Span(), typ.Span()), X: left, Type: typ}
			continue
		}
		op, prec, right, ok := p.infixOp()
		if !ok || prec < min {
			break
		}
		p.next()
		nextMin := prec + 1
		if right {
			nextMin = prec
		}
		rhs := p.parseBin(nextMin)
		left = &ast.InfixExpr{S: span.Merge(left.Span(), rhs.Span()), Op: op, Left: left, Right: rhs}
	}
	return left
}

func (p *Parser) infixOp() (string, token.Prec, bool, bool) {
	switch p.kind() {
	case token.Op:
		lit := p.lit()
		if token.IsAssignmentOp(lit) {
			return "", 0, false, false
		}
		prec, right := token.OpPrec(lit)
		return lit, prec, right, true
	case token.Ident:
		// A name between expressions is an alphanumeric operator (`eq`, `to`).
		prec, right := token.OpPrec(p.lit())
		return p.lit(), prec, right, true
	default:
		return "", 0, false, false
	}
}

func (p *Parser) parseUnary() ast.Expr {
	if p.kind() == token.Op && isPrefixOp(p.lit()) {
		op := p.lit()
		sp := p.span()
		p.next()
		x := p.parseUnary()
		return &ast.PrefixExpr{S: span.Merge(sp, x.Span()), Op: op, X: x}
	}
	return p.parsePostfix(p.parsePrimary())
}

func isPrefixOp(lit string) bool {
	switch lit {
	case "+", "-", "!", "~":
		return true
	default:
		return false
	}
}

func (p *Parser) parsePostfix(e ast.Expr) ast.Expr {
	if e == nil {
		e = p.badExpr()
	}
	for {
		switch p.kind() {
		case token.Dot:
			dot := p.span()
			p.next()
			switch p.kind() {
			case token.Ident:
				id := p.parseIdent()
				e = &ast.SelectExpr{S: span.Merge(e.Span(), id.S), X: e, Name: id.Name}
			case token.This:
				sp := p.span()
				p.next()
				qual := ""
				if id, ok := e.(*ast.Ident); ok {
					qual = id.Name
				}
				e = &ast.ThisExpr{S: span.Merge(e.Span(), sp), Qual: qual}
			case token.Super:
				sup := p.parseSuper()
				if id, ok := e.(*ast.Ident); ok {
					sup.Qual = id.Name
				}
				sup.S = span.Merge(e.Span(), sup.S)
				e = sup
			default:
				p.errAt(dot, "expected a member name after '.'")
				return e
			}
		case token.LParen:
			e = p.parseApply(e, nil)
		case token.LBrack:
			args := p.parseTypeArgs()
			if p.kind() == token.LParen {
				e = p.parseApply(e, args)
			} else {
				end := e.Span()
				if len(args) > 0 {
					end = args[len(args)-1].Span()
				}
				e = &ast.TypeApplyExpr{S: span.Merge(e.Span(), end), Fun: e, Args: args}
			}
		case token.LBrace:
			if p.tok().Newline {
				return e
			}
			blk := p.parseBlock()
			e = &ast.ApplyExpr{S: span.Merge(e.Span(), blk.S), Fun: e, Args: []ast.Expr{blk}}
		default:
			return e
		}
	}
}

func (p *Parser) parseApply(fun ast.Expr, targs []ast.TypeNode) ast.Expr {
	sp := p.span()
	p.expect(token.LParen)
	using := p.eat(token.Using)
	var args []ast.Expr
	if p.kind() != token.RParen {
		args = p.parseArgList()
	}
	end := p.span()
	p.expect(token.RParen)
	_ = sp
	return &ast.ApplyExpr{
		S:        span.Merge(fun.Span(), end),
		Fun:      fun,
		TypeArgs: targs,
		Args:     args,
		Using:    using,
	}
}

func (p *Parser) parseArgList() []ast.Expr {
	var args []ast.Expr
	for {
		p.skipSemis()
		if p.kind() == token.RParen || p.kind() == token.EOF {
			break
		}
		args = append(args, p.parseExpr())
		p.skipSemis()
		if !p.eat(token.Comma) {
			break
		}
	}
	return args
}

func (p *Parser) parsePrimary() ast.Expr {
	switch p.kind() {
	case token.Int, token.Float, token.String, token.Char, token.True, token.False, token.Null:
		return p.parseLit()
	case token.Ident:
		id := p.parseIdent()
		if id.Name == "_" && !id.Quoted {
			return &ast.WildcardExpr{S: id.S}
		}
		if p.kind() == token.Arrow {
			p.next()
			body := p.parseExpr()
			return &ast.LambdaExpr{
				S:      span.Merge(id.S, body.Span()),
				Params: []*ast.Param{{S: id.S, Name: id.Name}},
				Body:   body,
			}
		}
		return id
	case token.This:
		sp := p.span()
		p.next()
		return &ast.ThisExpr{S: sp}
	case token.Super:
		return p.parseSuper()
	case token.If:
		return p.parseIf()
	case token.While:
		return p.parseWhile()
	case token.Do:
		return p.parseDo()
	case token.For:
		return p.parseFor()
	case token.Try:
		return p.parseTry()
	case token.Throw:
		return p.parseThrow()
	case token.Return:
		return p.parseReturn()
	case token.New:
		return p.parseNew()
	case token.LParen:
		if p.looksLikeLambda() {
			return p.parseLambda()
		}
		return p.parseParenOrTuple()
	case token.LBrace:
		return p.parseBlock()
	case token.InterpStart:
		return p.parseInterp()
	default:
		p.err("expected expression")
		return p.badExpr()
	}
}

func (p *Parser) parseLit() ast.Expr {
	sp := p.span()
	switch p.kind() {
	case token.Int:
		t := p.lit()
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitInt, Text: t}
	case token.Float:
		t := p.lit()
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitFloat, Text: t}
	case token.String:
		t := p.lit()
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitString, Text: t}
	case token.Char:
		t := p.lit()
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitChar, Text: t}
	case token.True:
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitBool, Text: "true"}
	case token.False:
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitBool, Text: "false"}
	case token.Null:
		p.next()
		return &ast.LiteralExpr{S: sp, Kind: ast.LitNull, Text: "null"}
	default:
		p.err("expected literal")
		return p.badExpr()
	}
}

func (p *Parser) parseSuper() *ast.SuperExpr {
	sp := p.span()
	p.next()
	s := &ast.SuperExpr{S: sp}
	if p.eat(token.LBrack) {
		s.Mix = p.parseIdent().Name
		end := p.span()
		p.expect(token.RBrack)
		s.S = span.Merge(sp, end)
	}
	return s
}

func (p *Parser) parseParenOrTuple() ast.Expr {
	sp := p.span()
	p.expect(token.LParen)
	p.skipSemis()
	if p.eat(token.RParen) {
		return &ast.LiteralExpr{S: span.Merge(sp, p.prev.Span), Kind: ast.LitUnit, Text: "()"}
	}
	first := p.parseExpr()
	p.skipSemis()
	if p.eat(token.Comma) {
		elems := []ast.Expr{first}
		for {
			p.skipSemis()
			if p.kind() == token.RParen || p.kind() == token.EOF {
				break
			}
			elems = append(elems, p.parseExpr())
			p.skipSemis()
			if !p.eat(token.Comma) {
				break
			}
		}
		end := p.span()
		p.expect(token.RParen)
		return &ast.TupleExpr{S: span.Merge(sp, end), Elems: elems}
	}
	end := p.span()
	p.expect(token.RParen)
	return &ast.ParenExpr{S: span.Merge(sp, end), X: first}
}

func (p *Parser) looksLikeLambda() bool {
	if p.kind() == token.Ident && p.peekKind() == token.Arrow {
		return true
	}
	if p.kind() != token.LParen {
		return false
	}
	depth := 0
	for j := p.i; j < len(p.toks); j++ {
		switch p.toks[j].Kind {
		case token.LParen:
			depth++
		case token.RParen:
			depth--
			if depth == 0 {
				return j+1 < len(p.toks) && p.toks[j+1].Kind == token.Arrow
			}
		case token.EOF:
			return false
		}
		if depth < 0 {
			return false
		}
	}
	return false
}

func (p *Parser) parseLambda() ast.Expr {
	sp := p.span()
	p.expect(token.LParen)
	var params []*ast.Param
	if p.kind() != token.RParen {
		for {
			params = append(params, p.parseLambdaParam())
			if !p.eat(token.Comma) {
				break
			}
		}
	}
	p.expect(token.RParen)
	p.expect(token.Arrow)
	body := p.parseExpr()
	return &ast.LambdaExpr{S: span.Merge(sp, body.Span()), Params: params, Body: body}
}

func (p *Parser) parseLambdaParam() *ast.Param {
	id := p.parseIdent()
	param := &ast.Param{S: id.S, Name: id.Name}
	if p.eat(token.Colon) {
		param.Type = p.parseType()
	}
	return param
}

func (p *Parser) parseBlock() *ast.BlockExpr {
	sp := p.span()
	p.expect(token.LBrace)
	stats := p.parseStatsUntil(token.RBrace, false, 0)
	end := p.span()
	p.expect(token.RBrace)
	return &ast.BlockExpr{S: span.Merge(sp, end), Stats: stats}
}

func (p *Parser) parseStatsUntil(stop token.Kind, inEnum bool, bodyCol int) []ast.Stmt {
	var stats []ast.Stmt
	p.skipSemis()
	for p.kind() != stop && p.kind() != token.EOF && p.kind() != token.End {
		if bodyCol > 0 && p.col() < bodyCol {
			break
		}
		before := p.i
		stats = append(stats, p.parseStmt())
		p.skipSemis()
		if p.i == before {
			break
		}
	}
	return stats
}

func (p *Parser) parseIf() ast.Expr {
	sp := p.span()
	headerCol := p.col()
	p.next()
	var cond ast.Expr
	var keyLine int
	if p.eat(token.LParen) {
		cond = p.parseExpr()
		p.expect(token.RParen)
		p.skipSemis()
		if p.kind() == token.Then {
			keyLine = p.line()
			p.next()
		} else {
			keyLine = p.prev.Span.Position().Line
		}
	} else {
		cond = p.parseExpr()
		p.skipSemis()
		keyLine = p.line()
		if !p.expect(token.Then) {
			return &ast.IfExpr{S: sp, Cond: cond, Then: p.badExpr()}
		}
	}
	thenE := p.parseBodyAfter(headerCol, keyLine, token.Else)
	var elseE ast.Expr
	p.skipSemis()
	if p.kind() == token.Else {
		elseLine := p.line()
		elseCol := p.col()
		p.next()
		elseE = p.parseBodyAfter(elseCol, elseLine, token.EOF)
		_ = headerCol
	}
	end := thenE.Span()
	if elseE != nil {
		end = elseE.Span()
	}
	return &ast.IfExpr{S: span.Merge(sp, end), Cond: cond, Then: thenE, Else: elseE}
}

func (p *Parser) parseWhile() ast.Expr {
	sp := p.span()
	headerCol := p.col()
	p.next()
	var cond ast.Expr
	var keyLine int
	if p.eat(token.LParen) {
		cond = p.parseExpr()
		p.expect(token.RParen)
		keyLine = p.prev.Span.Position().Line
		p.skipSemis()
		if p.kind() == token.Do {
			keyLine = p.line()
			p.next()
		}
	} else {
		cond = p.parseExpr()
		p.skipSemis()
		keyLine = p.line()
		p.expect(token.Do)
	}
	body := p.parseBodyAfter(headerCol, keyLine, token.EOF)
	return &ast.WhileExpr{S: span.Merge(sp, body.Span()), Cond: cond, Body: body}
}

func (p *Parser) parseDo() ast.Expr {
	sp := p.span()
	headerCol := p.col()
	keyLine := p.line()
	p.next()
	body := p.parseBodyAfter(headerCol, keyLine, token.While)
	p.skipSemis()
	p.expect(token.While)
	cond := p.parseExpr()
	return &ast.DoWhileExpr{S: span.Merge(sp, cond.Span()), Body: body, Cond: cond}
}

func (p *Parser) parseFor() ast.Expr {
	sp := p.span()
	p.next()
	var enums []*ast.Enumerator
	if p.eat(token.LBrace) {
		p.skipSemis()
		for p.kind() != token.RBrace && p.kind() != token.EOF {
			enums = append(enums, p.parseEnumerator())
			p.skipSemis()
		}
		p.expect(token.RBrace)
	} else {
		enums = append(enums, p.parseEnumerator())
		for p.eat(token.Semi) {
			if p.kind() == token.Yield || p.kind() == token.Do || p.kind() == token.EOF {
				break
			}
			enums = append(enums, p.parseEnumerator())
		}
	}
	p.skipSemis()
	yield := true
	keyLine := p.line()
	keyCol := p.col()
	if p.eat(token.Do) {
		yield = false
	} else if !p.expect(token.Yield) {
		return &ast.ForExpr{S: sp, Enums: enums, Body: p.badExpr(), Yield: true}
	}
	body := p.parseBodyAfter(keyCol, keyLine, token.EOF)
	return &ast.ForExpr{S: span.Merge(sp, body.Span()), Enums: enums, Body: body, Yield: yield}
}

func (p *Parser) parseEnumerator() *ast.Enumerator {
	sp := p.span()
	if p.eat(token.If) {
		g := p.parseExpr()
		return &ast.Enumerator{S: span.Merge(sp, g.Span()), Guard: g}
	}
	pat := p.parsePattern()
	if p.eat(token.LArrow) {
		e := p.parseExpr()
		return &ast.Enumerator{S: span.Merge(pat.Span(), e.Span()), Pat: pat, Expr: e, IsGen: true}
	}
	if p.eat(token.Assign) {
		e := p.parseExpr()
		return &ast.Enumerator{S: span.Merge(pat.Span(), e.Span()), Pat: pat, Expr: e}
	}
	p.err("expected <- or = in enumerator")
	return &ast.Enumerator{S: pat.Span(), Pat: pat}
}

func (p *Parser) parseTry() ast.Expr {
	sp := p.span()
	headerCol := p.col()
	keyLine := p.line()
	p.next()
	body := p.parseBodyAfter(headerCol, keyLine, token.Catch)
	var catches []*ast.CaseClause
	p.skipSemis()
	if p.eat(token.Catch) {
		catches = p.parseCaseBlock()
	}
	var fin ast.Expr
	p.skipSemis()
	if p.eat(token.Finally) {
		fin = p.parseExpr()
	}
	end := body.Span()
	if fin != nil {
		end = fin.Span()
	} else if n := len(catches); n > 0 {
		end = catches[n-1].Span()
	}
	return &ast.TryExpr{S: span.Merge(sp, end), Body: body, Catches: catches, Finally: fin}
}

func (p *Parser) parseThrow() ast.Expr {
	sp := p.span()
	p.next()
	if p.endsAbrupt() {
		p.err("throw expects an expression")
		return &ast.ThrowExpr{S: sp, X: p.badExpr()}
	}
	x := p.parseExpr()
	return &ast.ThrowExpr{S: span.Merge(sp, x.Span()), X: x}
}

func (p *Parser) parseReturn() ast.Expr {
	sp := p.span()
	p.next()
	if p.endsAbrupt() {
		return &ast.ReturnExpr{S: sp}
	}
	x := p.parseExpr()
	return &ast.ReturnExpr{S: span.Merge(sp, x.Span()), X: x}
}

func (p *Parser) endsAbrupt() bool {
	switch p.kind() {
	case token.Semi, token.RBrace, token.EOF, token.Else, token.Case, token.Catch, token.Finally, token.End:
		return true
	default:
		return false
	}
}

func (p *Parser) parseNew() ast.Expr {
	sp := p.span()
	headerCol := p.col()
	p.next()
	var parents []*ast.Parent
	parents = append(parents, p.parseParent())
	for p.eat(token.With) {
		parents = append(parents, p.parseParent())
	}
	var tmpl *ast.Template
	if p.kind() == token.LBrace || p.kind() == token.Colon {
		tmpl, _ = p.parseMaybeTemplate(false, headerCol)
	}
	end := parents[len(parents)-1].S
	if tmpl != nil {
		end = tmpl.S
	}
	return &ast.NewExpr{S: span.Merge(sp, end), Parents: parents, Template: tmpl}
}

func (p *Parser) parseParent() *ast.Parent {
	sp := p.span()
	// A brace after a parent starts the template, not a refinement.
	// Parentheses restore refinement types: `extends (Foo { type T })`.
	p.noRefine++
	typ := p.parseType()
	p.noRefine--
	par := &ast.Parent{S: typ.Span(), Type: typ}
	if p.kind() == token.LParen {
		p.next()
		par.HasArgs = true
		if p.kind() != token.RParen {
			par.Args = p.parseArgList()
		}
		end := p.span()
		p.expect(token.RParen)
		par.S = span.Merge(sp, end)
	}
	return par
}

// stripSyntheticParen removes the parentheses the lexer wraps around a hole.
// A hole written as ${(1)} keeps the inner parentheses.
func stripSyntheticParen(e ast.Expr) ast.Expr {
	if p, ok := e.(*ast.ParenExpr); ok {
		return p.X
	}
	return e
}

func (p *Parser) parseInterp() ast.Expr {
	sp := p.span()
	name := p.lit()
	p.next()
	var parts []string
	var holes []ast.Expr
	for p.kind() != token.InterpEnd && p.kind() != token.EOF {
		if p.kind() == token.StrPart {
			parts = append(parts, p.lit())
			p.next()
			continue
		}
		holes = append(holes, stripSyntheticParen(p.parseExpr()))
	}
	end := p.span()
	p.expect(token.InterpEnd)
	if len(parts) == len(holes) {
		parts = append(parts, "")
	}
	if len(parts) == 0 {
		parts = []string{""}
	}
	return &ast.InterpExpr{S: span.Merge(sp, end), Name: name, Parts: parts, Holes: holes}
}

func (p *Parser) parseCaseBlock() []*ast.CaseClause {
	if p.kind() != token.LBrace {
		p.err("expected '{' to open cases")
		return nil
	}
	p.next()
	var cases []*ast.CaseClause
	p.skipSemis()
	for p.kind() != token.RBrace && p.kind() != token.EOF {
		if p.kind() != token.Case {
			p.err("expected case")
			p.next()
			p.skipSemis()
			continue
		}
		cases = append(cases, p.parseCase())
		p.skipSemis()
	}
	p.expect(token.RBrace)
	return cases
}

func (p *Parser) parseCase() *ast.CaseClause {
	sp := p.span()
	p.expect(token.Case)
	pat := p.parsePattern()
	var guard ast.Expr
	if p.eat(token.If) {
		guard = p.parseExpr()
	}
	p.expect(token.Arrow)
	body := p.parseCaseBody()
	return &ast.CaseClause{S: span.Merge(sp, body.Span()), Pat: pat, Guard: guard, Body: body}
}

func (p *Parser) parseCaseBody() ast.Expr {
	if p.kind() == token.Case || p.kind() == token.RBrace || p.kind() == token.EOF {
		p.err("expected case body")
		return p.badExpr()
	}
	var stats []ast.Stmt
	for p.kind() != token.Case && p.kind() != token.RBrace && p.kind() != token.EOF {
		before := p.i
		stats = append(stats, p.parseStmt())
		p.skipSemis()
		if p.i == before {
			break
		}
	}
	return unwrapStats(stats)
}

// parseBodyAfter parses the expression or indented block that follows a
// control keyword. headerLine is the line of that keyword. headerCol is the
// column a later line must pass to count as the body.
func (p *Parser) parseBodyAfter(headerCol, headerLine int, stop token.Kind) ast.Expr {
	p.skipSemis()
	if p.kind() == token.EOF || (stop != token.EOF && p.kind() == stop) {
		p.err("expected expression")
		return p.badExpr()
	}
	if p.line() == headerLine || p.col() <= headerCol {
		return p.parseExpr()
	}
	bodyCol := p.col()
	sp := p.span()
	var stats []ast.Stmt
	for p.kind() != token.EOF && p.kind() != token.End {
		if stop != token.EOF && p.kind() == stop && p.col() <= headerCol {
			break
		}
		if p.col() < bodyCol {
			break
		}
		before := p.i
		stats = append(stats, p.parseStmt())
		p.skipSemis()
		if p.i == before {
			break
		}
	}
	blk := unwrapStats(stats)
	if blk.Span().IsZero() {
		if b, ok := blk.(*ast.BlockExpr); ok {
			b.S = sp
		}
	}
	return blk
}

func unwrapStats(stats []ast.Stmt) ast.Expr {
	if len(stats) == 1 {
		if es, ok := stats[0].(*ast.ExprStmt); ok && es.X != nil {
			return es.X
		}
	}
	var sp span.Span
	if len(stats) > 0 && stats[0] != nil {
		sp = stats[0].Span()
		if last := stats[len(stats)-1]; last != nil {
			sp = span.Merge(sp, last.Span())
		}
	}
	return &ast.BlockExpr{S: sp, Stats: stats}
}
