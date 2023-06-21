// Package driver wires the pipeline a person actually runs:
// scan, parse, elaborate, and either evaluate or compile.
package driver

import (
	"fmt"
	"os"
	"strings"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/check"
	"github.com/Aneesh495/fathom/internal/compile"
	"github.com/Aneesh495/fathom/internal/diag"
	"github.com/Aneesh495/fathom/internal/eval"
	"github.com/Aneesh495/fathom/internal/parser"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/vm"
)

// Source is one compilation unit loaded from disk or from a test string.
type Source struct {
	Name string
	Text string
}

// Result is what run commands print.
type Result struct {
	Value  string
	Output string
	Diags  *diag.List
	Prog   *compile.Program
	Info   *check.Info
}

// Load parses sources in order. Later files see symbols entered by earlier ones.
func Load(sources []Source) (*check.Info, *diag.List, error) {
	var files []*ast.File
	all := diag.NewList(200)
	for _, src := range sources {
		file, diags := parser.Parse(span.NewFile(src.Name, src.Text))
		if diags != nil {
			for _, d := range diags.Items {
				all.Add(d)
			}
		}
		files = append(files, file)
	}
	if all.HasErrors() {
		return nil, all, fmt.Errorf("parse failed")
	}
	info := check.Check(files)
	if info.Diags != nil {
		for _, d := range info.Diags.Items {
			all.Add(d)
		}
	}
	if all.HasErrors() {
		return info, all, fmt.Errorf("check failed")
	}
	return info, all, nil
}

// ReadFile loads one source file.
func ReadFile(path string) (Source, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Source{}, err
	}
	return Source{Name: path, Text: string(b)}, nil
}

// Eval runs the reference walker.
func Eval(info *check.Info) (Result, error) {
	v, out, err := eval.Run(info)
	return Result{Value: v.String(), Output: out, Info: info}, err
}

// Compile lowers info and executes it on the bytecode machine.
func Compile(info *check.Info) (Result, error) {
	prog, err := compile.Compile(info)
	if err != nil {
		return Result{Info: info, Prog: prog}, err
	}
	got, err := vm.Run(prog)
	return Result{Value: got.Value, Output: got.Output, Info: info, Prog: prog}, err
}

// Expectation reads a leading comment of the form `// expect: ...` or
// `// expect-out: ...`. The value comparison ignores a trailing unit result
// when the interesting output is printed text.
type Expectation struct {
	Value  string
	Output string
	HasVal bool
	HasOut bool
}

// ParseExpect scans the first comment lines of src.
func ParseExpect(src string) Expectation {
	var exp Expectation
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			if rest, ok := strings.CutPrefix(trim, "// expect:"); ok {
				exp.Value = strings.TrimSpace(rest)
				exp.HasVal = true
			}
			if rest, ok := strings.CutPrefix(trim, "// expect-out:"); ok {
				exp.Output = strings.TrimSpace(rest)
				exp.HasOut = true
			}
			continue
		}
		break
	}
	return exp
}
