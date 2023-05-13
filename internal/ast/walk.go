package ast

// Walk visits n and its children in preorder.
// If fn returns false, the children of that node are not visited.
func Walk(n Node, fn func(Node) bool) {
	if n == nil || isNilNode(n) {
		return
	}
	if !fn(n) {
		return
	}
	switch n := n.(type) {
	case *File:
		for _, s := range n.Stats {
			Walk(s, fn)
		}
	case *Import:
	case *ClassDecl:
		walkStats(n.Body, fn)
	case *DefDecl:
		Walk(n.Body, fn)
	case *ValDecl:
		Walk(n.Body, fn)
		if n.Pat != nil {
			walkPat(n.Pat, fn)
		}
	case *GivenDecl:
		Walk(n.Body, fn)
		walkStats(n.BodyTemplate, fn)
	case *ExtDecl:
		walkStats(n.Body, fn)
	case *ExprStmt:
		Walk(n.X, fn)
	case *BlockExpr:
		for _, s := range n.Stats {
			Walk(s, fn)
		}
	case *IfExpr:
		Walk(n.Cond, fn)
		Walk(n.Then, fn)
		Walk(n.Else, fn)
	case *MatchExpr:
		Walk(n.Scrut, fn)
		for _, c := range n.Cases {
			Walk(c.Body, fn)
			Walk(c.Guard, fn)
		}
	case *WhileExpr:
		Walk(n.Cond, fn)
		Walk(n.Body, fn)
	case *DoWhileExpr:
		Walk(n.Body, fn)
		Walk(n.Cond, fn)
	case *ForExpr:
		for _, e := range n.Enums {
			Walk(e.Expr, fn)
			Walk(e.Guard, fn)
		}
		Walk(n.Body, fn)
	case *TryExpr:
		Walk(n.Body, fn)
		for _, c := range n.Catches {
			Walk(c.Body, fn)
		}
		Walk(n.Finally, fn)
	case *ThrowExpr:
		Walk(n.X, fn)
	case *ReturnExpr:
		Walk(n.X, fn)
	case *AssignExpr:
		Walk(n.LHS, fn)
		Walk(n.RHS, fn)
	case *LambdaExpr:
		Walk(n.Body, fn)
	case *SelectExpr:
		Walk(n.X, fn)
	case *ApplyExpr:
		Walk(n.Fun, fn)
		for _, a := range n.Args {
			Walk(a, fn)
		}
	case *TypeApplyExpr:
		Walk(n.Fun, fn)
	case *InfixExpr:
		Walk(n.Left, fn)
		Walk(n.Right, fn)
	case *PrefixExpr:
		Walk(n.X, fn)
	case *AscribeExpr:
		Walk(n.X, fn)
	case *InterpExpr:
		for _, h := range n.Holes {
			Walk(h, fn)
		}
	case *NewExpr:
		for _, p := range n.Parents {
			for _, a := range p.Args {
				Walk(a, fn)
			}
		}
		walkStats(n.Template, fn)
	case *ParenExpr:
		Walk(n.X, fn)
	case *TupleExpr:
		for _, e := range n.Elems {
			Walk(e, fn)
		}
	}
}

func walkStats(t *Template, fn func(Node) bool) {
	if t == nil {
		return
	}
	for _, s := range t.Stats {
		Walk(s, fn)
	}
}

func walkPat(p Pattern, fn func(Node) bool) {
	if p == nil || isNilNode(p) {
		return
	}
	if !fn(p) {
		return
	}
	switch n := p.(type) {
	case *BindPat:
		walkPat(n.Pat, fn)
	case *TypedPat:
		walkPat(n.Pat, fn)
	case *CtorPat:
		Walk(n.Ctor, fn)
		for _, a := range n.Pats {
			walkPat(a, fn)
		}
	case *TuplePat:
		for _, a := range n.Pats {
			walkPat(a, fn)
		}
	case *AltPat:
		for _, a := range n.Pats {
			walkPat(a, fn)
		}
	}
}

func isNilNode(n Node) bool {
	switch n.(type) {
	case nil:
		return true
	default:
		return n == nil
	}
}
