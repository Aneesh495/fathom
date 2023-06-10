package compile

import "github.com/Aneesh495/fathom/internal/ast"

// children lists the expression and statement children a free-variable scan needs.
func children(n ast.Node) []ast.Node {
	switch n := n.(type) {
	case *ast.ParenExpr:
		return []ast.Node{n.X}
	case *ast.TupleExpr:
		return exprs(n.Elems)
	case *ast.BlockExpr:
		return stmts(n.Stats)
	case *ast.IfExpr:
		return []ast.Node{n.Cond, n.Then, n.Else}
	case *ast.WhileExpr:
		return []ast.Node{n.Cond, n.Body}
	case *ast.DoWhileExpr:
		return []ast.Node{n.Body, n.Cond}
	case *ast.AssignExpr:
		return []ast.Node{n.LHS, n.RHS}
	case *ast.PrefixExpr:
		return []ast.Node{n.X}
	case *ast.InfixExpr:
		return []ast.Node{n.Left, n.Right}
	case *ast.SelectExpr:
		return []ast.Node{n.X}
	case *ast.ApplyExpr:
		out := []ast.Node{n.Fun}
		out = append(out, exprs(n.Args)...)
		return out
	case *ast.AscribeExpr:
		return []ast.Node{n.X}
	case *ast.ReturnExpr:
		return []ast.Node{n.X}
	case *ast.ThrowExpr:
		return []ast.Node{n.X}
	case *ast.MatchExpr:
		out := []ast.Node{n.Scrut}
		for _, cs := range n.Cases {
			out = append(out, cs.Pat, cs.Guard, cs.Body)
		}
		return out
	case *ast.InterpExpr:
		return exprs(n.Holes)
	case *ast.ExprStmt:
		return []ast.Node{n.X}
	case *ast.ValDecl:
		return []ast.Node{n.Body}
	default:
		return nil
	}
}

func exprs(es []ast.Expr) []ast.Node {
	out := make([]ast.Node, 0, len(es))
	for _, e := range es {
		out = append(out, e)
	}
	return out
}

func stmts(ss []ast.Stmt) []ast.Node {
	out := make([]ast.Node, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}
