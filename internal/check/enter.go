package check

import (
	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/types"
)

func (ck *Checker) enterFile(f *ast.File) {
	scope := ck.Root
	if f.Package != nil && len(f.Package.Parts) > 0 {
		scope = ck.enterPackage(f.Package.Parts, ck.Root)
	}
	prev := ck.scope
	ck.scope = scope
	for _, im := range f.Imports {
		ck.bindImport(im)
	}
	for _, st := range f.Stats {
		ck.enterStmt(st, ck.scope)
	}
	ck.scope = prev
}

func (ck *Checker) enterPackage(parts []string, parent *types.Scope) *types.Scope {
	scope := parent
	for _, name := range parts {
		var pkg *types.Symbol
		for _, s := range scope.Local(name) {
			if s.Kind == types.SymPackage {
				pkg = s
				break
			}
		}
		if pkg == nil {
			pkg = ck.Ctx.NewSymbol(name, types.SymPackage)
			pkg.Scope = types.NewScope(scope, pkg)
			scope.Bind(pkg)
		}
		if pkg.Scope == nil {
			pkg.Scope = types.NewScope(scope, pkg)
		}
		scope = pkg.Scope
	}
	return scope
}

func (ck *Checker) bindImport(im *ast.Import) {
	if im == nil || im.Path == nil || len(im.Path.Parts) == 0 {
		return
	}
	// Walk from the root so imports are absolute within this compilation.
	scope := ck.Root
	var last *types.Symbol
	for i, name := range im.Path.Parts {
		hits := scope.Lookup(name)
		if i == len(im.Path.Parts)-1 && len(im.Selectors) == 0 && !im.Wildcard {
			if s := preferType(hits); s != nil {
				last = s
			} else if s := preferTerm(hits); s != nil {
				last = s
			}
			break
		}
		if s := firstPackageOrType(hits); s != nil && s.Scope != nil {
			last = s
			scope = s.Scope
			continue
		}
		if s := preferTerm(hits); s != nil {
			last = s
		}
		break
	}
	if len(im.Selectors) == 0 && !im.Wildcard {
		if last != nil {
			ck.scope.Bind(last)
		} else if len(im.Path.Parts) > 0 {
			ck.errorf(im.Span(), "not found: %s", im.Path.String())
		}
		return
	}
	base := last
	if base == nil || base.Scope == nil {
		ck.errorf(im.Span(), "not found: %s", im.Path.String())
		return
	}
	if im.Wildcard {
		for _, name := range base.Scope.Names() {
			for _, s := range base.Scope.Local(name) {
				ck.scope.Bind(s)
			}
		}
	}
	for _, sel := range im.Selectors {
		hits := base.Scope.Local(sel.Name)
		s := preferTerm(hits)
		if s == nil {
			s = preferType(hits)
		}
		if s == nil {
			ck.errorf(sel.Span, "not found: %s", sel.Name)
			continue
		}
		if sel.Rename != "" && sel.Rename != sel.Name {
			alias := *s
			alias.Name = sel.Rename
			ck.scope.Bind(&alias)
			continue
		}
		ck.scope.Bind(s)
	}
}

func firstPackageOrType(syms []*types.Symbol) *types.Symbol {
	for _, s := range syms {
		if s.Kind == types.SymPackage || s.IsClassLike() {
			return s
		}
	}
	return nil
}

func (ck *Checker) enterStmt(st ast.Stmt, scope *types.Scope) {
	if st == nil {
		return
	}
	prev := ck.scope
	ck.scope = scope
	defer func() { ck.scope = prev }()
	switch n := st.(type) {
	case *ast.ClassDecl:
		ck.enterClass(n, scope)
	case *ast.DefDecl:
		ck.enterDef(n, scope)
	case *ast.ValDecl:
		ck.enterVal(n, scope)
	case *ast.TypeDecl:
		ck.enterType(n, scope)
	case *ast.GivenDecl:
		ck.enterGiven(n, scope)
	case *ast.ExtDecl:
		ck.enterExt(n, scope)
	case *ast.ExprStmt:
		// expressions are checked in the body pass
	default:
	}
}

func (ck *Checker) enterClass(n *ast.ClassDecl, scope *types.Scope) *types.Symbol {
	kind := types.SymClass
	flags := types.Flags(0)
	switch n.Kind {
	case ast.KindTrait:
		kind = types.SymTrait
		flags |= types.FlagAbstract
	case ast.KindObject:
		kind = types.SymObject
		flags |= types.FlagModule | types.FlagFinal
	case ast.KindEnum:
		kind = types.SymTrait
		flags |= types.FlagAbstract | types.FlagSealed
	}
	if ast.HasMod(n.Mods, ast.ModAbstract) {
		flags |= types.FlagAbstract
	}
	if ast.HasMod(n.Mods, ast.ModSealed) || n.Kind == ast.KindEnum {
		flags |= types.FlagSealed
	}
	if ast.HasMod(n.Mods, ast.ModFinal) || n.Kind == ast.KindObject {
		flags |= types.FlagFinal
	}
	if ast.HasMod(n.Mods, ast.ModCase) {
		flags |= types.FlagCase
	}
	if ast.HasMod(n.Mods, ast.ModOpen) {
		flags &^= types.FlagFinal
	}
	sym := ck.Ctx.NewSymbol(n.Name, kind)
	sym.Flags = flags
	sym.Span = n.Span()
	sym.AST = n
	sym.Scope = types.NewScope(scope, sym)
	scope.Bind(sym)
	ck.noteSym(n, sym)
	ck.bindTypeParams(n.TParams, sym, sym.Scope)
	if n.HasParams {
		if sym.Has(types.FlagCase) {
			for _, p := range n.Params {
				if p != nil && !p.IsVar {
					p.IsVal = true
				}
			}
		}
		sym.Ctor = ck.enterParams(n.Params, sym, true)
	}
	if n.Kind == ast.KindEnum {
		ck.enterEnumCases(n, sym, scope)
	}
	if ast.HasMod(n.Mods, ast.ModCase) && n.Kind == ast.KindClass {
		ck.ensureCompanion(sym, scope)
	}
	if n.Body != nil {
		for _, st := range n.Body.Stats {
			ck.enterStmt(st, sym.Scope)
		}
	}
	for _, ann := range n.Anns {
		if ann.Name == "main" && sym.Kind == types.SymObject {
			sym.Flags |= types.FlagMain
		}
	}
	return sym
}

func (ck *Checker) bindTypeParams(list []*ast.TypeParam, owner *types.Symbol, scope *types.Scope) {
	for _, tp := range list {
		if tp == nil {
			continue
		}
		s := ck.Ctx.NewSymbol(tp.Name, types.SymTypeParam)
		s.Span = tp.Span()
		s.AST = tp
		switch tp.Variance {
		case ast.Covariant:
			s.Variance = types.Covariant
			s.Flags |= types.FlagCovariant
		case ast.Contravariant:
			s.Variance = types.Contravariant
			s.Flags |= types.FlagContravariant
		}
		s.Skolem = ck.Ctx.Skolem(tp.Name, ck.Ctx.Bot, ck.Ctx.Top)
		owner.AddMember(s)
		owner.TParams = append(owner.TParams, s)
		scope.Bind(s)
		ck.noteSym(tp, s)
	}
}

func (ck *Checker) enterParams(ps []*ast.Param, owner *types.Symbol, ctor bool) []*types.Symbol {
	var out []*types.Symbol
	for _, p := range ps {
		if p == nil {
			continue
		}
		s := ck.Ctx.NewSymbol(p.Name, types.SymTerm)
		s.Span = p.Span()
		s.AST = p
		s.Flags = types.FlagParam | types.FlagStable
		if p.IsVar {
			s.Flags |= types.FlagMutable
			s.Flags &^= types.FlagStable
		}
		if ctor && (p.IsVal || p.IsVar) {
			owner.AddMember(s)
			if owner.Scope != nil {
				owner.Scope.Bind(s)
			}
		}
		out = append(out, s)
		ck.noteSym(p, s)
	}
	return out
}

func (ck *Checker) ensureCompanion(class *types.Symbol, scope *types.Scope) *types.Symbol {
	if class.Companion != nil {
		return class.Companion
	}
	for _, s := range scope.Local(class.Name) {
		if s.Kind == types.SymObject {
			class.Companion = s
			s.Companion = class
			return s
		}
	}
	obj := ck.Ctx.NewSymbol(class.Name, types.SymObject)
	obj.Flags = types.FlagModule | types.FlagSynthetic | types.FlagFinal
	obj.Scope = types.NewScope(scope, obj)
	obj.Companion = class
	class.Companion = obj
	scope.Bind(obj)
	return obj
}

func (ck *Checker) enterEnumCases(n *ast.ClassDecl, parent *types.Symbol, outer *types.Scope) {
	comp := ck.ensureCompanion(parent, outer)
	for _, c := range n.Cases {
		if c == nil {
			continue
		}
		if !c.HasParams {
			obj := ck.Ctx.NewSymbol(c.Name, types.SymObject)
			obj.Flags = types.FlagCase | types.FlagModule | types.FlagFinal
			obj.Span = c.Span()
			obj.AST = c
			obj.Scope = types.NewScope(comp.Scope, obj)
			comp.AddMember(obj)
			comp.Scope.Bind(obj)
			parent.Scope.Bind(obj)
			outer.Bind(obj)
			ck.noteSym(c, obj)
			continue
		}
		cls := ck.Ctx.NewSymbol(c.Name, types.SymClass)
		cls.Flags = types.FlagCase | types.FlagFinal
		cls.Span = c.Span()
		cls.AST = c
		cls.Scope = types.NewScope(parent.Scope, cls)
		// Inherit the enum's type parameters so Cons[A] tracks List[A].
		for _, tp := range classParams(parent) {
			clone := ck.Ctx.NewSymbol(tp.Name, types.SymTypeParam)
			clone.Variance = tp.Variance
			clone.Flags = tp.Flags
			clone.Skolem = ck.Ctx.Skolem(tp.Name, ck.Ctx.Bot, ck.Ctx.Top)
			cls.AddMember(clone)
			cls.Scope.Bind(clone)
		}
		for _, p := range c.Params {
			if p != nil && !p.IsVar {
				p.IsVal = true
			}
		}
		cls.Ctor = ck.enterParams(c.Params, cls, true)
		comp.AddMember(cls)
		comp.Scope.Bind(cls)
		parent.Scope.Bind(cls)
		outer.Bind(cls)
		ck.noteSym(c, cls)
	}
}

func (ck *Checker) enterDef(n *ast.DefDecl, scope *types.Scope) *types.Symbol {
	sym := ck.Ctx.NewSymbol(n.Name, types.SymMethod)
	sym.Span = n.Span()
	sym.AST = n
	if ast.HasMod(n.Mods, ast.ModOverride) {
		sym.Flags |= types.FlagOverride
	}
	if ast.HasMod(n.Mods, ast.ModAbstract) || n.Body == nil {
		sym.Flags |= types.FlagAbstract
	}
	if ast.HasMod(n.Mods, ast.ModFinal) {
		sym.Flags |= types.FlagFinal
	}
	if ast.HasMod(n.Mods, ast.ModInfix) {
		sym.Flags |= types.FlagExtension
	}
	for _, ann := range n.Anns {
		if ann.Name == "main" {
			sym.Flags |= types.FlagMain
		}
	}
	if n.Name == "main" && scope == ck.Root || (scope != nil && scope.Owner == nil) {
		// top-level main is an entry if no @main was seen yet
	}
	scope.Bind(sym)
	if scope.Owner != nil && scope.Owner.IsClassLike() {
		scope.Owner.AddMember(sym)
	}
	sym.Scope = types.NewScope(scope, sym)
	ck.bindTypeParams(n.TParams, sym, sym.Scope)
	for _, list := range n.Lists {
		if list == nil {
			continue
		}
		ps := ck.enterParams(list.Params, sym, false)
		for _, p := range ps {
			sym.Scope.Bind(p)
		}
		sym.Lists = append(sym.Lists, types.ParamList{Using: list.Using, Params: ps})
	}
	ck.noteSym(n, sym)
	top := scope == nil || scope.Owner == nil || scope.Owner.Kind == types.SymPackage
	if sym.Has(types.FlagMain) || (n.Name == "main" && top) {
		ck.main = sym
	}
	return sym
}

func (ck *Checker) enterVal(n *ast.ValDecl, scope *types.Scope) *types.Symbol {
	name := n.Name
	if name == "" && n.Pat != nil {
		name = "_pat"
	}
	sym := ck.Ctx.NewSymbol(name, types.SymTerm)
	sym.Span = n.Span()
	sym.AST = n
	sym.Flags = types.FlagStable
	if n.Mutable {
		sym.Flags |= types.FlagMutable
		sym.Flags &^= types.FlagStable
	}
	if ast.HasMod(n.Mods, ast.ModLazy) {
		sym.Flags |= types.FlagSynthetic
	}
	scope.Bind(sym)
	if scope.Owner != nil && scope.Owner.IsClassLike() && n.Pat == nil {
		scope.Owner.AddMember(sym)
	}
	ck.noteSym(n, sym)
	return sym
}

func (ck *Checker) enterType(n *ast.TypeDecl, scope *types.Scope) *types.Symbol {
	sym := ck.Ctx.NewSymbol(n.Name, types.SymType)
	sym.Span = n.Span()
	sym.AST = n
	if n.Body == nil {
		sym.Flags |= types.FlagAbstract
	}
	if ast.HasMod(n.Mods, ast.ModOpaque) {
		sym.Flags |= types.FlagOpaque
	}
	sym.Scope = types.NewScope(scope, sym)
	ck.bindTypeParams(n.TParams, sym, sym.Scope)
	scope.Bind(sym)
	if scope.Owner != nil && scope.Owner.IsClassLike() {
		scope.Owner.AddMember(sym)
	}
	ck.noteSym(n, sym)
	return sym
}

func (ck *Checker) enterGiven(n *ast.GivenDecl, scope *types.Scope) *types.Symbol {
	name := n.Name
	if name == "" {
		name = "given"
	}
	sym := ck.Ctx.NewSymbol(name, types.SymTerm)
	sym.Flags = types.FlagGiven | types.FlagStable
	sym.Span = n.Span()
	sym.AST = n
	sym.Scope = types.NewScope(scope, sym)
	ck.bindTypeParams(n.TParams, sym, sym.Scope)
	for _, list := range n.Lists {
		if list == nil {
			continue
		}
		ps := ck.enterParams(list.Params, sym, false)
		for _, p := range ps {
			sym.Scope.Bind(p)
		}
		sym.Lists = append(sym.Lists, types.ParamList{Using: list.Using, Params: ps})
	}
	scope.Bind(sym)
	ck.given = append(ck.given, sym)
	ck.noteSym(n, sym)
	if n.BodyTemplate != nil {
		for _, st := range n.BodyTemplate.Stats {
			ck.enterStmt(st, sym.Scope)
			if d, ok := st.(*ast.DefDecl); ok {
				if m := ck.syms[d]; m != nil {
					sym.AddMember(m)
				}
			}
		}
	}
	return sym
}

func (ck *Checker) enterExt(n *ast.ExtDecl, scope *types.Scope) {
	ext := &extension{Scope: scope}
	if len(n.TParams) > 0 {
		owner := ck.Ctx.NewSymbol("extension", types.SymMethod)
		owner.Scope = types.NewScope(scope, owner)
		ck.bindTypeParams(n.TParams, owner, owner.Scope)
		ext.TParams = classParams(owner)
		scope = owner.Scope
	}
	if n.Param != nil {
		ps := ck.enterParams([]*ast.Param{n.Param}, ck.Ctx.NewSymbol("ext", types.SymMethod), false)
		if len(ps) == 1 {
			ext.Param = ps[0]
		}
	}
	if n.Body != nil {
		recvScope := types.NewScope(scope, ext.Param)
		if ext.Param != nil {
			recvScope.Bind(ext.Param)
		}
		for _, st := range n.Body.Stats {
			if d, ok := st.(*ast.DefDecl); ok {
				m := ck.enterDef(d, recvScope)
				m.Flags |= types.FlagExtension
				ext.Methods = append(ext.Methods, m)
			} else {
				ck.enterStmt(st, recvScope)
			}
		}
	}
	ck.exts = append(ck.exts, ext)
}
