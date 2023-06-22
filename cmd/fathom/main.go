package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Aneesh495/fathom/internal/compile"
	"github.com/Aneesh495/fathom/internal/driver"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "run", "eval", "check", "dis":
		if len(args) != 2 {
			return usage(stderr)
		}
		return command(args[0], args[1], stdout, stderr)
	case "help", "-h", "--help":
		return usage(stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func command(cmd, path string, stdout, stderr io.Writer) error {
	src, err := driver.ReadFile(path)
	if err != nil {
		return err
	}
	info, diags, err := driver.Load([]driver.Source{src})
	if diags != nil && diags.ErrorCount() > 0 {
		fmt.Fprintln(stderr, diags.Render())
	}
	if err != nil {
		return err
	}
	switch cmd {
	case "check":
		fmt.Fprintf(stdout, "checked %s\n", path)
		return nil
	case "eval":
		res, err := driver.Eval(info)
		writeResult(stdout, res.Value, res.Output)
		return err
	case "dis":
		res, err := driver.Compile(info)
		if res.Prog != nil {
			for _, fn := range res.Prog.Funcs {
				fmt.Fprintln(stdout, compile.Disasm(fn))
			}
		}
		return err
	default:
		res, err := driver.Compile(info)
		writeResult(stdout, res.Value, res.Output)
		return err
	}
}

func writeResult(w io.Writer, value, output string) {
	if output != "" {
		fmt.Fprint(w, output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Fprintln(w)
		}
	}
	if value != "" && value != "()" {
		fmt.Fprintln(w, value)
	}
}

func usage(w io.Writer) error {
	fmt.Fprint(w, `fathom is a Scala 3 kernel.

Usage:
  fathom run <file.scala>     compile and execute on the bytecode machine
  fathom eval <file.scala>    execute with the reference walker
  fathom check <file.scala>   parse and typecheck
  fathom dis <file.scala>     print bytecode

`)
	return nil
}
