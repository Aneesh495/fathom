// Package check elaborates Fathom's surface syntax into symbols and types.
//
// Three passes keep forward references honest. Enter creates every symbol.
// Signatures resolve parents, parameters, and result types. Bodies check
// expressions only after every name in the file has a type.
package check

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/diag"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/types"
)

// Info is the result of a successful or partial check.
// Types and Syms are keyed by AST node identity.
type Info struct {
	Ctx      *types.Context
	Root     *types.Scope
	Types    map[ast.Node]*types.Type
	Syms     map[ast.Node]*types.Symbol
	Main     *types.Symbol
	Diags    *diag.List
	Files    []*ast.File
	Evidence map[ast.Node][]*types.Symbol
	// Rewrites holds desugarings, such as for comprehensions, that were typechecked
	// in place of the original node.
	Rewrites map[ast.Node]ast.Expr
}

// Checker is the elaboration state. It is safe for one compilation at a time.
type Checker struct {
	Ctx       *types.Context
	Diags     *diag.List
	Root      *types.Scope
	scope     *types.Scope
	this      *types.Symbol
	ret       *types.Type
	level     int
	types     map[ast.Node]*types.Type
	syms      map[ast.Node]*types.Symbol
	exts      []*extension
	given     []*types.Symbol
	givenOpen map[*types.Type]bool
	busy      map[*types.Symbol]bool
	main      *types.Symbol
	files     []*ast.File
	evidence  map[ast.Node][]*types.Symbol
	rewrites  map[ast.Node]ast.Expr
	subject   ast.Node
}

type extension struct {
	TParams []*types.Symbol
	Param   *types.Symbol
	Scope   *types.Scope
	Methods []*types.Symbol
}

// New starts a checker with the universe and tuple classes in scope.
func New(diags *diag.List) *Checker {
	if diags == nil {
		diags = diag.NewList(100)
	}
	ctx := types.NewContext()
	root := ctx.InitUniverse()
	return &Checker{
		Ctx:      ctx,
		Diags:    diags,
		Root:     root,
		scope:    root,
		types:    map[ast.Node]*types.Type{},
		syms:     map[ast.Node]*types.Symbol{},
		busy:     map[*types.Symbol]bool{},
		evidence: map[ast.Node][]*types.Symbol{},
		rewrites: map[ast.Node]ast.Expr{},
	}
}

// Check elaborates files as one program. Prelude should already be included
// by the driver when the caller wants the standard definitions.
func Check(files []*ast.File) *Info {
	ck := New(diag.NewList(120))
	ck.files = files
	for _, f := range files {
		ck.enterFile(f)
	}
	for _, f := range files {
		ck.sigFile(f)
	}
	for _, f := range files {
		ck.bodyFile(f)
	}
	return &Info{
		Ctx: ck.Ctx, Root: ck.Root, Types: ck.types, Syms: ck.syms,
		Main: ck.main, Diags: ck.Diags, Files: files, Evidence: ck.evidence, Rewrites: ck.rewrites,
	}
}

func (ck *Checker) errorf(sp span.Span, format string, args ...any) {
	ck.Diags.Errorf(sp, format, args...)
}

func (ck *Checker) noteType(n ast.Node, t *types.Type) *types.Type {
	if n != nil && t != nil {
		ck.types[n] = t
	}
	return t
}

func (ck *Checker) noteSym(n ast.Node, s *types.Symbol) {
	if n != nil && s != nil {
		ck.syms[n] = s
	}
}

func (ck *Checker) mismatch(sp span.Span, found, want *types.Type) {
	ck.errorf(sp, "type mismatch\n  found:    %s\n  required: %s", ck.Ctx.Format(found), ck.Ctx.Format(want))
}

func (ck *Checker) push(owner *types.Symbol) *types.Scope {
	prev := ck.scope
	ck.scope = types.NewScope(prev, owner)
	return prev
}

func (ck *Checker) lookup(name string) []*types.Symbol {
	if ck.scope == nil {
		return nil
	}
	return ck.scope.Lookup(name)
}

func preferType(syms []*types.Symbol) *types.Symbol {
	var obj *types.Symbol
	for _, s := range syms {
		switch s.Kind {
		case types.SymClass, types.SymTrait, types.SymType, types.SymTypeParam:
			return s
		case types.SymObject:
			if obj == nil {
				obj = s
			}
		}
	}
	return obj
}

func preferTerm(syms []*types.Symbol) *types.Symbol {
	var class *types.Symbol
	for _, s := range syms {
		switch s.Kind {
		case types.SymTerm, types.SymMethod, types.SymObject, types.SymPackage:
			return s
		case types.SymClass, types.SymTrait:
			if class == nil {
				class = s
			}
		}
	}
	return class
}

func (ck *Checker) resolveType(n ast.TypeNode) *types.Type {
	if n == nil {
		return ck.Ctx.Error
	}
	if t, ok := ck.types[n]; ok {
		return t
	}
	t := ck.resolveType1(n)
	return ck.noteType(n, t)
}

func (ck *Checker) resolveType1(n ast.TypeNode) *types.Type {
	switch n := n.(type) {
	case *ast.IdentType:
		return ck.resolveIdentType(n)
	case *ast.SelectType:
		return ck.resolveSelectType(n)
	case *ast.PathType:
		return ck.resolvePathType(n.Prefix, n.Name, n.Span())
	case *ast.ApplyType:
		return ck.resolveApplyType(n)
	case *ast.FunType:
		params := make([]*types.Type, len(n.Params))
		for i, p := range n.Params {
			params[i] = ck.resolveType(p)
		}
		return ck.Ctx.Fun(params, ck.resolveType(n.Result))
	case *ast.TupleType:
		return ck.tupleType(n.Elems)
	case *ast.InfixType:
		l := ck.resolveType(n.Left)
		r := ck.resolveType(n.Right)
		switch n.Op {
		case "|":
			return ck.Ctx.Union(l, r)
		case "&":
			return ck.Ctx.Inter(l, r)
		default:
			ck.errorf(n.Span(), "unknown type operator %s", n.Op)
			return ck.Ctx.Error
		}
	case *ast.SingletonType:
		return ck.singletonOf(n.Path)
	case *ast.ByNameType:
		return ck.resolveType(n.X)
	case *ast.ProjectType:
		base := ck.resolveType(n.X)
		return ck.project(base, n.Name, n.Span())
	case *ast.WildType:
		lo, hi := ck.Ctx.Bot, ck.Ctx.Top
		if n.Lo != nil {
			lo = ck.resolveType(n.Lo)
		}
		if n.Hi != nil {
			hi = ck.resolveType(n.Hi)
		}
		return ck.Ctx.Skolem("_", lo, hi)
	case *ast.LitType:
		return ck.litType(n.Lit)
	case *ast.RefineType:
		return ck.resolveType(n.X)
	default:
		ck.errorf(n.Span(), "unsupported type")
		return ck.Ctx.Error
	}
}

func (ck *Checker) resolveIdentType(n *ast.IdentType) *types.Type {
	sym := preferType(ck.lookup(n.Name))
	if sym == nil {
		ck.errorf(n.Span(), "not found: type %s", n.Name)
		return ck.Ctx.Error
	}
	ck.noteSym(n, sym)
	return ck.typeSym(sym, nil, n.Span())
}

func (ck *Checker) typeSym(sym *types.Symbol, args []*types.Type, sp span.Span) *types.Type {
	switch sym.Kind {
	case types.SymTypeParam:
		if sym.Skolem == nil {
			ck.errorf(sp, "type parameter %s is not yet bound", sym.Name)
			return ck.Ctx.Error
		}
		return sym.Skolem
	case types.SymType:
		if sym.Has(types.FlagAbstract) {
			if ck.this != nil {
				return ck.Ctx.Select(&types.Path{Head: ck.this}, sym.Name)
			}
			if sym.Result != nil {
				return sym.Result
			}
			return ck.Ctx.Top
		}
		if sym.Type != nil {
			return sym.Type
		}
		return ck.Ctx.Error
	case types.SymClass, types.SymTrait, types.SymObject:
		if sym == ck.Ctx.Any {
			return ck.Ctx.Top
		}
		if sym == ck.Ctx.Nothing {
			return ck.Ctx.Bot
		}
		need := len(classParams(sym))
		if need == 0 {
			return ck.Ctx.Named(sym, nil)
		}
		if len(args) != need {
			ck.errorf(sp, "%s expects %d type argument(s)", sym.Name, need)
			return ck.Ctx.Error
		}
		return ck.Ctx.Named(sym, args)
	default:
		ck.errorf(sp, "%s is not a type", sym.Name)
		return ck.Ctx.Error
	}
}

func classParams(sym *types.Symbol) []*types.Symbol {
	if sym == nil {
		return nil
	}
	var out []*types.Symbol
	for _, m := range sym.Members {
		if m.Kind == types.SymTypeParam {
			out = append(out, m)
		}
	}
	return out
}

func (ck *Checker) resolveApplyType(n *ast.ApplyType) *types.Type {
	sym, base := ck.typeSymbol(n.Fun)
	if sym == nil {
		return ck.Ctx.Error
	}
	args := make([]*types.Type, len(n.Args))
	for i, a := range n.Args {
		args[i] = ck.resolveType(a)
	}
	if base != nil && sym.Kind == types.SymType {
		return base
	}
	return ck.typeSym(sym, args, n.Span())
}

func (ck *Checker) typeSymbol(n ast.TypeNode) (*types.Symbol, *types.Type) {
	switch n := n.(type) {
	case *ast.IdentType:
		sym := preferType(ck.lookup(n.Name))
		if sym == nil {
			ck.errorf(n.Span(), "not found: type %s", n.Name)
			return nil, ck.Ctx.Error
		}
		return sym, nil
	case *ast.SelectType:
		t := ck.resolveSelectType(n)
		if t != nil && t.Kind == types.TyNamed {
			return t.Sym, t
		}
		return nil, t
	default:
		t := ck.resolveType(n)
		if t != nil && t.Kind == types.TyNamed {
			return t.Sym, t
		}
		ck.errorf(n.Span(), "expected a type constructor")
		return nil, ck.Ctx.Error
	}
}

func (ck *Checker) resolveSelectType(n *ast.SelectType) *types.Type {
	if id, ok := n.X.(*ast.IdentType); ok {
		syms := ck.lookup(id.Name)
		if term := preferTerm(syms); term != nil && (term.Kind == types.SymTerm || term.Kind == types.SymObject) && preferType(syms) == nil {
			return ck.resolvePathType(termToExpr(term, id.Span()), n.Name, n.Span())
		}
		if term := preferTerm(syms); term != nil && term.Kind == types.SymTerm && preferType(syms) != nil && preferType(syms).Kind == types.SymObject {
			// A value shadows a type of another kind. Prefer the value for a selection
			// that is a type member.
			if preferType(syms).Name == id.Name && preferType(syms).Kind != types.SymClass && preferType(syms).Kind != types.SymTrait {
				return ck.resolvePathType(&ast.Ident{S: id.Span(), Name: id.Name}, n.Name, n.Span())
			}
		}
	}
	qual := ck.resolveType(n.X)
	if qual.IsError() {
		return qual
	}
	if qual.Kind == types.TyNamed && qual.Sym != nil {
		if m := qual.Sym.Get(n.Name); m != nil && (m.Kind == types.SymType || m.Kind == types.SymClass || m.Kind == types.SymTrait || m.Kind == types.SymObject) {
			return ck.typeSym(m, nil, n.Span())
		}
		if comp := qual.Sym.Companion; comp != nil {
			if m := comp.Get(n.Name); m != nil && m.IsType() {
				return ck.typeSym(m, nil, n.Span())
			}
		}
	}
	ck.errorf(n.Span(), "%s is not a member of %s", n.Name, ck.Ctx.Format(qual))
	return ck.Ctx.Error
}

func termToExpr(sym *types.Symbol, sp span.Span) ast.Expr {
	return &ast.Ident{S: sp, Name: sym.Name}
}

func (ck *Checker) resolvePathType(prefix ast.Expr, name string, sp span.Span) *types.Type {
	path, ok := ck.stablePath(prefix)
	if !ok {
		ck.errorf(sp, "type selection requires a stable path")
		return ck.Ctx.Error
	}
	return ck.Ctx.Select(path, name)
}

func (ck *Checker) project(base *types.Type, name string, sp span.Span) *types.Type {
	if base.Kind == types.TyNamed && base.Sym != nil {
		if m := ck.Ctx.LookupMember(base, name); m != nil && m.Kind == types.SymType {
			if m.Type != nil && !m.Has(types.FlagAbstract) {
				return m.Type
			}
			if m.Result != nil {
				return m.Result
			}
		}
	}
	ck.errorf(sp, "no type member %s in %s", name, ck.Ctx.Format(base))
	return ck.Ctx.Error
}

func (ck *Checker) tupleType(elems []ast.TypeNode) *types.Type {
	args := make([]*types.Type, len(elems))
	for i, e := range elems {
		args[i] = ck.resolveType(e)
	}
	switch len(args) {
	case 2:
		return ck.Ctx.Named(ck.Ctx.Tuple2, args)
	case 3:
		return ck.Ctx.Named(ck.Ctx.Tuple3, args)
	case 4:
		return ck.Ctx.Named(ck.Ctx.Tuple4, args)
	default:
		return ck.Ctx.Error
	}
}

func (ck *Checker) litType(lit *ast.LiteralExpr) *types.Type {
	if lit == nil {
		return ck.Ctx.Error
	}
	switch lit.Kind {
	case ast.LitInt:
		return ck.Ctx.IntType()
	case ast.LitFloat:
		return ck.Ctx.DoubleType()
	case ast.LitBool:
		return ck.Ctx.BooleanType()
	case ast.LitString:
		return ck.Ctx.StringType()
	case ast.LitChar:
		return ck.Ctx.CharType()
	case ast.LitUnit:
		return ck.Ctx.UnitType()
	case ast.LitNull:
		return ck.Ctx.NullType()
	default:
		return ck.Ctx.Error
	}
}

func (ck *Checker) singletonOf(e ast.Expr) *types.Type {
	path, ok := ck.stablePath(e)
	if !ok {
		ck.errorf(e.Span(), "expected a stable path")
		return ck.Ctx.Error
	}
	return ck.Ctx.Singleton(path)
}

func (ck *Checker) stablePath(e ast.Expr) (*types.Path, bool) {
	switch e := e.(type) {
	case *ast.Ident:
		sym := preferTerm(ck.lookup(e.Name))
		if sym == nil {
			return nil, false
		}
		if sym.Kind == types.SymTerm && sym.Has(types.FlagStable) {
			return &types.Path{Head: sym}, true
		}
		if sym.Kind == types.SymObject {
			return &types.Path{Head: sym}, true
		}
		if sym.Name == "this" {
			return &types.Path{Head: sym}, true
		}
		return nil, false
	case *ast.ThisExpr:
		if ck.this == nil {
			return nil, false
		}
		return &types.Path{Head: ck.this}, true
	case *ast.SelectExpr:
		path, ok := ck.stablePath(e.X)
		if !ok {
			return nil, false
		}
		path = path.Clone()
		path.Tail = append(path.Tail, e.Name)
		return path, true
	default:
		return nil, false
	}
}

func (ck *Checker) ensure(cond bool, sp span.Span, msg string) {
	if !cond {
		ck.errorf(sp, "%s", msg)
	}
}

func (ck *Checker) fmtSym(s *types.Symbol) string {
	if s == nil {
		return "?"
	}
	return s.Name
}

// classArgs maps a class type's parameters to the arguments of t.
func classArgMap(t *types.Type) map[*types.Type]*types.Type {
	t = types.Repr(t)
	m := map[*types.Type]*types.Type{}
	if t == nil || t.Kind != types.TyNamed || t.Sym == nil {
		return m
	}
	params := classParams(t.Sym)
	if len(params) != len(t.Args) {
		return m
	}
	for i, p := range params {
		if p.Skolem != nil {
			m[p.Skolem] = t.Args[i]
		}
	}
	return m
}

// inheritSubst maps owner's type parameters to the arguments recv implies.
// recv may be a subclass; the arguments are read from the parent clause that names owner.
func (ck *Checker) inheritSubst(recv *types.Type, owner *types.Symbol) map[*types.Type]*types.Type {
	recv = types.Repr(recv)
	if recv == nil || owner == nil || recv.Kind != types.TyNamed {
		return classArgMap(recv)
	}
	if recv.Sym == owner {
		return classArgMap(recv)
	}
	base := classArgMap(recv)
	if recv.Sym == nil {
		return base
	}
	for _, p := range recv.Sym.Parents {
		inst := ck.Ctx.Subst(p, base)
		inst = types.Repr(inst)
		if inst != nil && inst.Kind == types.TyNamed && inst.Sym == owner {
			return classArgMap(inst)
		}
		if inst != nil && inst.Kind == types.TyNamed && inst.Sym != recv.Sym {
			if m := ck.inheritSubst(inst, owner); len(m) > 0 {
				return m
			}
		}
	}
	return base
}

// caseSubst maps a case class's skolems onto the scrutinee's arguments.
// Cons extends List[A'] with its own A'. Matching against List[T] sends A' to T.
func (ck *Checker) caseSubst(caseSym *types.Symbol, scrut *types.Type) map[*types.Type]*types.Type {
	out := map[*types.Type]*types.Type{}
	scrut = types.Repr(scrut)
	if caseSym == nil || scrut == nil || scrut.Kind != types.TyNamed {
		return out
	}
	if scrut.Sym == caseSym {
		return classArgMap(scrut)
	}
	for _, p := range caseSym.Parents {
		p = types.Repr(p)
		if p == nil || p.Kind != types.TyNamed || p.Sym != scrut.Sym || len(p.Args) != len(scrut.Args) {
			continue
		}
		for i := range p.Args {
			arg := types.Repr(p.Args[i])
			if arg != nil && arg.Kind == types.TyVar && arg.Skolem {
				out[arg] = scrut.Args[i]
			}
		}
		return out
	}
	return out
}

func substSymType(c *types.Context, t *types.Type, m map[*types.Type]*types.Type) *types.Type {
	if len(m) == 0 || t == nil {
		return t
	}
	return c.Subst(t, m)
}
