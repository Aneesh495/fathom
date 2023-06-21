package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	paths, err := filepath.Glob("../../examples/*.scala")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no examples")
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			exp := ParseExpect(string(text))
			info, diags, err := Load([]Source{{Name: path, Text: string(text)}})
			if err != nil {
				t.Fatalf("load: %v\n%s", err, diags.Render())
			}
			got, err := Compile(info)
			if err != nil {
				t.Fatal(err)
			}
			if exp.HasVal && strings.TrimSpace(got.Value) != exp.Value {
				t.Fatalf("value %q want %q", got.Value, exp.Value)
			}
			if exp.HasOut && strings.TrimSpace(got.Output) != exp.Output {
				t.Fatalf("output %q want %q", got.Output, exp.Output)
			}
			// The reference walker must agree.
			ref, err := Eval(info)
			if err != nil {
				t.Fatal(err)
			}
			if ref.Value != got.Value || ref.Output != got.Output {
				t.Fatalf("eval %q %q vm %q %q", ref.Value, ref.Output, got.Value, got.Output)
			}
		})
	}
}
