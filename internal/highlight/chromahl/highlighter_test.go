package chromahl

import (
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/highlight"
)

var samples = map[string]string{
	"main.go": `package main

import "fmt"

// Greet says hello.
func Greet(name string) (int, error) {
	if name == "" {
		return 0, fmt.Errorf("empty %q", name)
	}
	s := ` + "`raw\nmultiline`" + `
	fmt.Println(s, 0x1F, 3.14, len(name))
	return 1, nil
}
`,
	"app.ts": `import { a } from "./a";
/* block
   comment */
export class Foo<T> extends Bar {
  private x: number = 1;
  async run(): Promise<void> { await a?.b(` + "`tpl ${x}\n`" + `); }
}
`,
	"script.py": `#!/usr/bin/env python
import os

@decorator
def f(x, *args, **kw):
    """Docstring
    over lines."""
    return [i for i in range(10) if i and not x] # trailing
`,
	"lib.rs": `use std::io;

/// Doc.
#[derive(Debug)]
pub struct S<'a> { x: &'a str }

fn main() {
    let v: Vec<i32> = vec![1, 2, 3];
    println!("{}", v.len());
}
`,
	"data.json":        "{\n  \"a\": [1, 2.5, true, null],\n  \"b\": {\"c\": \"d\"}\n}\n",
	"README.md":        "# Title\n\nSome *emph* and `code`.\n\n```go\nfmt.Println(1)\n```\n\n- item\n- [link](http://x)\n",
	"ci.yaml":          "name: ci\non:\n  push:\n    branches: [main]\njobs:\n  b:\n    runs-on: ubuntu\n    steps:\n      - run: echo \"hi\" # c\n",
	"notes.zzzunknown": "just some text\n\twith tabs\n\nand blanks\n",
	"index.html":       "<!doctype html>\n<html><body class=\"x\"><script>var a = 1;</script></body></html>\n",
	"Makefile":         "all:\n\tgo build ./...\n",
}

func TestLinesInvariant(t *testing.T) {
	h := New()
	for path, src := range samples {
		t.Run(path, func(t *testing.T) {
			want := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
			got := h.Lines(path, src)
			if len(got) != len(want) {
				t.Fatalf("got %d lines, want %d", len(got), len(want))
			}
			for i, toks := range got {
				var b strings.Builder
				for j, tk := range toks {
					if tk.Text == "" {
						t.Errorf("line %d has an empty token", i)
					}
					if strings.Contains(tk.Text, "\n") {
						t.Errorf("line %d token contains a newline: %q", i, tk.Text)
					}
					if j > 0 && toks[j-1].Kind == tk.Kind {
						t.Errorf("line %d: adjacent tokens share kind %d: %q %q", i, tk.Kind, toks[j-1].Text, tk.Text)
					}
					b.WriteString(tk.Text)
				}
				if b.String() != want[i] {
					t.Errorf("line %d: tokens give %q, want %q", i, b.String(), want[i])
				}
			}
		})
	}
}

func TestLinesEdgeCases(t *testing.T) {
	h := New()
	tests := []struct {
		name, path, src string
		want            []string
	}{
		{"empty", "a.go", "", nil},
		{"only newline", "a.go", "\n", []string{""}},
		{"no trailing newline", "a.go", "package a", []string{"package a"}},
		{"blank lines", "a.go", "package a\n\n\nvar x = 1\n", []string{"package a", "", "", "var x = 1"}},
		{"two trailing newlines", "a.go", "package a\n\n", []string{"package a", ""}},
		{"crlf", "a.go", "package a\r\n\r\nvar x = 1\r\n", []string{"package a", "", "var x = 1"}},
		{"crlf unknown", "x.zzzunknown", "a\r\nb\r\n", []string{"a", "b"}},
		{"unknown no newline", "x.zzzunknown", "a\nb", []string{"a", "b"}},
		{"unicode", "a.go", "var s = \"héllo wörld 日本\"\n", []string{"var s = \"héllo wörld 日本\""}},
		{"unterminated string", "a.go", "var s = \"abc\nvar t = 1\n", []string{"var s = \"abc", "var t = 1"}},
		{"unterminated comment", "a.go", "/* never closed\nstill\n", []string{"/* never closed", "still"}},
		{"nested dirs", "pkg/sub/a.go", "package a\n", []string{"package a"}},
		{"windows path", `pkg\sub\a.go`, "package a\n", []string{"package a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.Lines(tt.path, tt.src)
			if len(got) != len(tt.want) {
				t.Fatalf("lines = %d, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, toks := range got {
				var b strings.Builder
				for _, tk := range toks {
					b.WriteString(tk.Text)
				}
				if b.String() != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, b.String(), tt.want[i])
				}
			}
		})
	}
}

func kindsOf(toks []highlight.Token) map[string]highlight.Kind {
	m := map[string]highlight.Kind{}
	for _, t := range toks {
		m[t.Text] = t.Kind
	}
	return m
}

func TestKindsGo(t *testing.T) {
	h := New()
	lines := h.Lines("main.go", samples["main.go"])
	// Line 5: func Greet(name string) (int, error) {
	k := kindsOf(lines[5])
	want := map[string]highlight.Kind{
		"func":   highlight.Keyword,
		"Greet":  highlight.Function,
		"string": highlight.Type,
		"int":    highlight.Type,
	}
	for text, kind := range want {
		if k[text] != kind {
			t.Errorf("%q kind = %d, want %d (%v)", text, k[text], kind, lines[5])
		}
	}
	if k := kindsOf(lines[4]); k["// Greet says hello."] != highlight.Comment {
		t.Errorf("comment line = %v", lines[4])
	}
	if k := kindsOf(lines[2]); k["import"] != highlight.Keyword || k["\"fmt\""] != highlight.String {
		t.Errorf("import line = %v", lines[2])
	}
	body := kindsOf(lines[11]) // fmt.Println(s, 0x1F, 3.14, len(name))
	if body["0x1F"] != highlight.Number || body["3.14"] != highlight.Number || body["len"] != highlight.Function {
		t.Errorf("call line = %v", lines[11])
	}
	if k := kindsOf(lines[12]); k["nil"] != highlight.Constant || k["return"] != highlight.Keyword {
		t.Errorf("return line = %v", lines[12])
	}
}

func TestKindsOthers(t *testing.T) {
	h := New()
	if k := kindsOf(h.Lines("a.json", "{\"k\": 1}\n")[0]); k["1"] != highlight.Number || k["\"k\""] == highlight.Plain {
		t.Errorf("json kinds = %v", k)
	}
	if k := kindsOf(h.Lines("a.html", "<div class=\"x\">\n")[0]); k["div"] != highlight.Tag || k["class"] != highlight.Attribute {
		t.Errorf("html kinds = %v", k)
	}
	if k := kindsOf(h.Lines("a.c", "#include <stdio.h>\nint x;\n")[0]); k["#include"] != highlight.Preprocessor {
		t.Errorf("c kinds = %v", k)
	}
	for _, tk := range h.Lines("a.zzzunknown", "plain text here\n")[0] {
		if tk.Kind != highlight.Plain {
			t.Errorf("unknown language token %+v", tk)
		}
	}
}

func TestLanguage(t *testing.T) {
	h := New()
	tests := map[string]string{
		"main.go":             "go",
		"src/app.ts":          "typescript",
		"a/b/script.py":       "python",
		"lib.rs":              "rust",
		"data.json":           "json",
		"x.zzzunknown":        "",
		"":                    "",
		"Makefile":            "makefile",
		`C:\repo\main.go`:     "go",
		"deep/dir/Dockerfile": "docker",
		"go.mod":              "",
	}
	for path, want := range tests {
		if got := h.Language(path); got != want {
			t.Errorf("Language(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestKindOfCategories(t *testing.T) {
	// Every chroma token type must map without panicking.
	for tt := range 9000 {
		_ = kindOf(chromaType(tt))
	}
}

func TestConcurrentUse(t *testing.T) {
	h := New()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 20; j++ {
				for path, src := range samples {
					h.Lines(path, src)
				}
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestOversizedIsPlain(t *testing.T) {
	big := strings.Repeat("var x = 1\n", maxHighlightBytes/10+10)
	got := New().Lines("a.go", big)
	if len(got) == 0 || len(got[0]) != 1 || got[0][0].Kind != highlight.Plain {
		t.Errorf("oversized source not plain: %+v", got[0])
	}
}

func goSource(n int) string {
	var b strings.Builder
	b.WriteString("package big\n\nimport \"fmt\"\n\n")
	for i := 0; b.Len() < n*40; i++ {
		b.WriteString("// Func")
		b.WriteString(strings.Repeat("x", i%5))
		b.WriteString(" does a thing.\nfunc Func(a int, b string) (string, error) {\n\tif a > 0 && b != \"\" {\n\t\treturn fmt.Sprintf(\"%d-%s\", a, b), nil\n\t}\n\treturn \"\", fmt.Errorf(\"bad: %v\", a)\n}\n\n")
	}
	return b.String()
}

func BenchmarkLines2000Go(b *testing.B) {
	src := goSource(2000)
	n := strings.Count(src, "\n")
	if n < 2000 {
		src += strings.Repeat("var x = 1\n", 2000-n)
	}
	src = strings.Join(strings.SplitN(src, "\n", 2001)[:2000], "\n") + "\n"
	h := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := h.Lines("big.go", src); len(got) != 2000 {
			b.Fatalf("lines = %d", len(got))
		}
	}
}

func BenchmarkLanguageCached(b *testing.B) {
	h := New()
	h.Language("a/b/c.go")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = h.Language("a/b/c.go")
	}
}
