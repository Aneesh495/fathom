package scanner

import "github.com/Aneesh495/fathom/internal/token"

// insertSemis turns statement-separating newlines into semicolon tokens.
// A newline is separating when the previous token can finish a statement and
// the next token does not continue one. See Token.CanEndStmt and
// Token.ContinuesLine.
func insertSemis(in []token.Token) []token.Token {
	if len(in) == 0 {
		return in
	}
	out := make([]token.Token, 0, len(in)+8)
	for i, t := range in {
		if t.Newline && i > 0 {
			prev := in[i-1]
			if prev.Kind.CanEndStmt() && !t.Kind.ContinuesLine() && t.Kind != token.EOF {
				semi := token.Token{
					Kind:     token.Semi,
					Inserted: true,
					Span:     prev.Span,
				}
				// Anchor the virtual semicolon on the end of the previous token
				// so the caret points at the line that actually ended.
				if prev.Span.File != nil {
					semi.Span.Start = prev.Span.End
					semi.Span.End = prev.Span.End
				}
				out = append(out, semi)
			}
		}
		// Newline has done its job. Keep the flag so tests can still see it,
		// but a second pass will not run.
		out = append(out, t)
	}
	return out
}
