package ast

import (
	"fmt"
	"strings"
)

// Sprint prints a compact s-expression. Positions are omitted so tests can
// match structure. The spelling is stable: do not reorder children.
func Sprint(n Node) string {
	var b strings.Builder
	write(&b, n)
	return b.String()
}

func write(b *strings.Builder, n Node) {
	if n == nil {
		b.WriteString("()")
		return
	}
	switch n := n.(type) {
	case *File:
		b.WriteString("(file")
		if n.Package != nil {
			b.WriteByte(' ')
			write(b, n.Package)
		}
		for _, im := range n.Imports {
			b.WriteByte(' ')
			writeImport(b, im)
		}
		for _, s := range n.Stats {
			b.WriteByte(' ')
			write(b, s)
		}
		b.WriteByte(')')
	case *QualName:
		fmt.Fprintf(b, "(qual %s)", n.String())
	case *Ident:
		if n.Quoted {
			fmt.Fprintf(b, "(id ` %s)", n.Name)
		} else {
			b.WriteString(n.Name)
		}
	case *LiteralExpr:
		writeLit(b, n)
	case *ParenExpr:
		b.WriteString("(paren ")
		write(b, n.X)
		b.WriteByte(')')
	case *TupleExpr:
		b.WriteString("(tuple")
		for _, e := range n.Elems {
			b.WriteByte(' ')
			write(b, e)
		}
		b.WriteByte(')')
	case *BlockExpr:
		b.WriteString("(block")
		for _, s := range n.Stats {
			b.WriteByte(' ')
			write(b, s)
		}
		b.WriteByte(')')
	case *IfExpr:
		b.WriteString("(if ")
		write(b, n.Cond)
		b.WriteByte(' ')
		write(b, n.Then)
		if n.Else != nil {
			b.WriteByte(' ')
			write(b, n.Else)
		}
		b.WriteByte(')')
	case *MatchExpr:
		b.WriteString("(match ")
		write(b, n.Scrut)
		for _, c := range n.Cases {
			b.WriteByte(' ')
			writeCase(b, c)
		}
		b.WriteByte(')')
	case *WhileExpr:
		b.WriteString("(while ")
		write(b, n.Cond)
		b.WriteByte(' ')
		write(b, n.Body)
		b.WriteByte(')')
	case *DoWhileExpr:
		b.WriteString("(do ")
		write(b, n.Body)
		b.WriteByte(' ')
		write(b, n.Cond)
		b.WriteByte(')')
	case *ForExpr:
		if n.Yield {
			b.WriteString("(for-yield")
		} else {
			b.WriteString("(for-do")
		}
		for _, e := range n.Enums {
			b.WriteByte(' ')
			writeEnum(b, e)
		}
		b.WriteByte(' ')
		write(b, n.Body)
		b.WriteByte(')')
	case *TryExpr:
		b.WriteString("(try ")
		write(b, n.Body)
		for _, c := range n.Catches {
			b.WriteByte(' ')
			writeCase(b, c)
		}
		if n.Finally != nil {
			b.WriteString(" (finally ")
			write(b, n.Finally)
			b.WriteByte(')')
		}
		b.WriteByte(')')
	case *ThrowExpr:
		b.WriteString("(throw ")
		write(b, n.X)
		b.WriteByte(')')
	case *ReturnExpr:
		b.WriteString("(return")
		if n.X != nil {
			b.WriteByte(' ')
			write(b, n.X)
		}
		b.WriteByte(')')
	case *AssignExpr:
		fmt.Fprintf(b, "(assign %s ", n.Op)
		write(b, n.LHS)
		b.WriteByte(' ')
		write(b, n.RHS)
		b.WriteByte(')')
	case *LambdaExpr:
		b.WriteString("(lam ")
		writeParams(b, n.Params)
		b.WriteByte(' ')
		write(b, n.Body)
		b.WriteByte(')')
	case *SelectExpr:
		b.WriteString("(sel ")
		write(b, n.X)
		b.WriteByte(' ')
		b.WriteString(n.Name)
		b.WriteByte(')')
	case *ApplyExpr:
		b.WriteString("(app ")
		write(b, n.Fun)
		if len(n.TypeArgs) > 0 {
			b.WriteString(" @[")
			for i, t := range n.TypeArgs {
				if i > 0 {
					b.WriteByte(' ')
				}
				write(b, t)
			}
			b.WriteByte(']')
		}
		if n.Using {
			b.WriteString(" using")
		}
		for _, a := range n.Args {
			b.WriteByte(' ')
			write(b, a)
		}
		b.WriteByte(')')
	case *TypeApplyExpr:
		b.WriteString("(tapply ")
		write(b, n.Fun)
		for _, t := range n.Args {
			b.WriteByte(' ')
			write(b, t)
		}
		b.WriteByte(')')
	case *InfixExpr:
		fmt.Fprintf(b, "(infix %s ", n.Op)
		write(b, n.Left)
		b.WriteByte(' ')
		write(b, n.Right)
		b.WriteByte(')')
	case *PrefixExpr:
		fmt.Fprintf(b, "(prefix %s ", n.Op)
		write(b, n.X)
		b.WriteByte(')')
	case *ThisExpr:
		if n.Qual != "" {
			fmt.Fprintf(b, "(this %s)", n.Qual)
		} else {
			b.WriteString("this")
		}
	case *SuperExpr:
		fmt.Fprintf(b, "(super %s %s)", n.Qual, n.Mix)
	case *AscribeExpr:
		b.WriteString("(ascribe ")
		write(b, n.X)
		b.WriteByte(' ')
		write(b, n.Type)
		b.WriteByte(')')
	case *InterpExpr:
		fmt.Fprintf(b, "(interp %s", n.Name)
		for i, part := range n.Parts {
			fmt.Fprintf(b, " %q", part)
			if i < len(n.Holes) {
				b.WriteByte(' ')
				write(b, n.Holes[i])
			}
		}
		b.WriteByte(')')
	case *NewExpr:
		b.WriteString("(new")
		for _, par := range n.Parents {
			b.WriteByte(' ')
			writeParent(b, par)
		}
		if n.Template != nil {
			b.WriteByte(' ')
			writeTemplate(b, n.Template)
		}
		b.WriteByte(')')
	case *WildcardExpr:
		b.WriteString("_")
	case *ExprStmt:
		write(b, n.X)
	case *ClassDecl:
		fmt.Fprintf(b, "(%s %s", n.Kind.String(), n.Name)
		if len(n.TParams) > 0 {
			b.WriteByte(' ')
			writeTParams(b, n.TParams)
		}
		if n.HasParams {
			b.WriteByte(' ')
			writeParams(b, n.Params)
		}
		for _, par := range n.Parents {
			b.WriteByte(' ')
			writeParent(b, par)
		}
		for _, c := range n.Cases {
			b.WriteByte(' ')
			writeEnumCase(b, c)
		}
		if n.Body != nil && len(n.Body.Stats) > 0 {
			b.WriteByte(' ')
			writeTemplate(b, n.Body)
		}
		b.WriteByte(')')
	case *DefDecl:
		fmt.Fprintf(b, "(def %s", n.Name)
		if len(n.TParams) > 0 {
			b.WriteByte(' ')
			writeTParams(b, n.TParams)
		}
		for _, list := range n.Lists {
			b.WriteByte(' ')
			if list.Using {
				b.WriteString("(using ")
				writeParams(b, list.Params)
				b.WriteByte(')')
			} else {
				writeParams(b, list.Params)
			}
		}
		if n.Result != nil {
			b.WriteString(" : ")
			write(b, n.Result)
		}
		if n.Body != nil {
			b.WriteByte(' ')
			write(b, n.Body)
		}
		b.WriteByte(')')
	case *ValDecl:
		if n.Mutable {
			b.WriteString("(var ")
		} else {
			b.WriteString("(val ")
		}
		if n.Pat != nil {
			writePat(b, n.Pat)
		} else {
			b.WriteString(n.Name)
		}
		if n.Type != nil {
			b.WriteString(" : ")
			write(b, n.Type)
		}
		if n.Body != nil {
			b.WriteByte(' ')
			write(b, n.Body)
		}
		b.WriteByte(')')
	case *TypeDecl:
		fmt.Fprintf(b, "(type %s", n.Name)
		if n.Body != nil {
			b.WriteString(" = ")
			write(b, n.Body)
		}
		if n.Lo != nil {
			b.WriteString(" >: ")
			write(b, n.Lo)
		}
		if n.Hi != nil {
			b.WriteString(" <: ")
			write(b, n.Hi)
		}
		b.WriteByte(')')
	case *GivenDecl:
		b.WriteString("(given")
		if n.Name != "" {
			b.WriteByte(' ')
			b.WriteString(n.Name)
		}
		if n.Type != nil {
			b.WriteString(" : ")
			write(b, n.Type)
		}
		if n.Body != nil {
			b.WriteByte(' ')
			write(b, n.Body)
		}
		if n.BodyTemplate != nil {
			b.WriteByte(' ')
			writeTemplate(b, n.BodyTemplate)
		}
		b.WriteByte(')')
	case *ExtDecl:
		b.WriteString("(extension ")
		if n.Param != nil {
			writeParams(b, []*Param{n.Param})
		}
		if n.Body != nil {
			b.WriteByte(' ')
			writeTemplate(b, n.Body)
		}
		b.WriteByte(')')
	case *Import:
		writeImport(b, n)
	case *IdentType:
		b.WriteString(n.Name)
	case *SelectType:
		b.WriteString("(tsel ")
		write(b, n.X)
		b.WriteByte(' ')
		b.WriteString(n.Name)
		b.WriteByte(')')
	case *ProjectType:
		b.WriteString("(project ")
		write(b, n.X)
		b.WriteByte(' ')
		b.WriteString(n.Name)
		b.WriteByte(')')
	case *ApplyType:
		b.WriteString("(tapp ")
		write(b, n.Fun)
		for _, a := range n.Args {
			b.WriteByte(' ')
			write(b, a)
		}
		b.WriteByte(')')
	case *FunType:
		b.WriteString("(=>")
		for _, p := range n.Params {
			b.WriteByte(' ')
			write(b, p)
		}
		b.WriteString(" -> ")
		write(b, n.Result)
		b.WriteByte(')')
	case *TupleType:
		b.WriteString("(ttuple")
		for _, e := range n.Elems {
			b.WriteByte(' ')
			write(b, e)
		}
		b.WriteByte(')')
	case *InfixType:
		fmt.Fprintf(b, "(tinfix %s ", n.Op)
		write(b, n.Left)
		b.WriteByte(' ')
		write(b, n.Right)
		b.WriteByte(')')
	case *RefineType:
		b.WriteString("(refine ")
		write(b, n.X)
		for _, s := range n.Stats {
			b.WriteByte(' ')
			write(b, s)
		}
		b.WriteByte(')')
	case *SingletonType:
		b.WriteString("(singleton ")
		write(b, n.Path)
		b.WriteByte(')')
	case *PathType:
		b.WriteString("(path ")
		write(b, n.Prefix)
		b.WriteByte(' ')
		b.WriteString(n.Name)
		b.WriteByte(')')
	case *ByNameType:
		b.WriteString("(byname ")
		write(b, n.X)
		b.WriteByte(')')
	case *WildType:
		b.WriteString("?")
	case *LitType:
		writeLit(b, n.Lit)
	default:
		if p, ok := n.(Pattern); ok {
			writePat(b, p)
			return
		}
		fmt.Fprintf(b, "(unknown %T)", n)
	}
}

func writeLit(b *strings.Builder, n *LiteralExpr) {
	if n == nil {
		b.WriteString("()")
		return
	}
	switch n.Kind {
	case LitString:
		fmt.Fprintf(b, "%q", n.Text)
	case LitChar:
		fmt.Fprintf(b, "(char %q)", n.Text)
	case LitBool:
		b.WriteString(n.Text)
	case LitNull:
		b.WriteString("null")
	case LitUnit:
		b.WriteString("()")
	default:
		b.WriteString(n.Text)
	}
}

func writeCase(b *strings.Builder, c *CaseClause) {
	b.WriteString("(case ")
	writePat(b, c.Pat)
	if c.Guard != nil {
		b.WriteString(" if ")
		write(b, c.Guard)
	}
	b.WriteString(" => ")
	write(b, c.Body)
	b.WriteByte(')')
}

func writeEnum(b *strings.Builder, e *Enumerator) {
	if e.Guard != nil && e.Pat == nil {
		b.WriteString("(guard ")
		write(b, e.Guard)
		b.WriteByte(')')
		return
	}
	if e.IsGen {
		b.WriteString("(gen ")
	} else {
		b.WriteString("(bind ")
	}
	writePat(b, e.Pat)
	b.WriteByte(' ')
	write(b, e.Expr)
	b.WriteByte(')')
}

func writePat(b *strings.Builder, p Pattern) {
	if p == nil {
		b.WriteString("_")
		return
	}
	switch n := p.(type) {
	case *WildPat:
		b.WriteString("_")
	case *BindPat:
		if n.Pat != nil {
			fmt.Fprintf(b, "(at %s ", n.Name)
			writePat(b, n.Pat)
			b.WriteByte(')')
		} else {
			b.WriteString(n.Name)
		}
	case *LitPat:
		writeLit(b, n.Lit)
	case *TypedPat:
		b.WriteString("(typed ")
		writePat(b, n.Pat)
		b.WriteByte(' ')
		write(b, n.Type)
		b.WriteByte(')')
	case *CtorPat:
		b.WriteString("(ctor ")
		write(b, n.Ctor)
		for _, a := range n.Pats {
			b.WriteByte(' ')
			writePat(b, a)
		}
		b.WriteByte(')')
	case *TuplePat:
		b.WriteString("(ptuple")
		for _, a := range n.Pats {
			b.WriteByte(' ')
			writePat(b, a)
		}
		b.WriteByte(')')
	case *AltPat:
		b.WriteString("(alt")
		for _, a := range n.Pats {
			b.WriteByte(' ')
			writePat(b, a)
		}
		b.WriteByte(')')
	default:
		fmt.Fprintf(b, "(pat %T)", p)
	}
}

func writeParams(b *strings.Builder, ps []*Param) {
	b.WriteString("[")
	for i, p := range ps {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(p.Name)
		if p.Type != nil {
			b.WriteString(": ")
			write(b, p.Type)
		}
	}
	b.WriteString("]")
}

func writeTParams(b *strings.Builder, ps []*TypeParam) {
	b.WriteString("[")
	for i, p := range ps {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(p.Variance.String())
		b.WriteString(p.Name)
	}
	b.WriteString("]")
}

func writeParent(b *strings.Builder, p *Parent) {
	b.WriteString("(parent ")
	write(b, p.Type)
	if p.HasArgs {
		for _, a := range p.Args {
			b.WriteByte(' ')
			write(b, a)
		}
	}
	b.WriteByte(')')
}

func writeTemplate(b *strings.Builder, t *Template) {
	b.WriteString("(template")
	for _, s := range t.Stats {
		b.WriteByte(' ')
		write(b, s)
	}
	b.WriteByte(')')
}

func writeEnumCase(b *strings.Builder, c *EnumCase) {
	fmt.Fprintf(b, "(case %s", c.Name)
	if c.HasParams {
		b.WriteByte(' ')
		writeParams(b, c.Params)
	}
	b.WriteByte(')')
}

func writeImport(b *strings.Builder, im *Import) {
	b.WriteString("(import ")
	if im.Path != nil {
		b.WriteString(im.Path.String())
	}
	for _, s := range im.Selectors {
		if s.Rename != "" && s.Rename != s.Name {
			fmt.Fprintf(b, " (%s => %s)", s.Name, s.Rename)
		} else if s.Name != "_" {
			b.WriteByte(' ')
			b.WriteString(s.Name)
		}
	}
	if im.Wildcard {
		b.WriteString(" _")
	}
	b.WriteByte(')')
}
