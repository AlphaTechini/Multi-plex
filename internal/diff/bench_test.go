package diff

import (
	"fmt"
	"strings"
	"testing"
)

// synthDiff builds a diff of about n lines: hunks of 3 context lines, a run
// of 2 changed lines and 3 context lines, spread over files of 10 hunks.
func synthDiff(n int) string {
	var b strings.Builder
	lines, file := 0, 0
	for lines < n {
		file++
		fmt.Fprintf(&b, "diff --git a/pkg/file%d.go b/pkg/file%d.go\nindex 1111111..2222222 100644\n--- a/pkg/file%d.go\n+++ b/pkg/file%d.go\n", file, file, file, file)
		for h := 0; h < 10 && lines < n; h++ {
			start := 1 + h*40
			fmt.Fprintf(&b, "@@ -%d,8 +%d,8 @@ func handler%d(ctx context.Context, req *Request) error {\n", start, start, h)
			b.WriteString(" \tctx = trace.With(ctx, \"handler\")\n \tlogger := log.From(ctx)\n \tif req == nil {\n")
			fmt.Fprintf(&b, "-\t\treturn fmt.Errorf(\"nil request %d\")\n-\tresult, err := store.Get(ctx, req.ID, opts...)\n", h)
			fmt.Fprintf(&b, "+\t\treturn fmt.Errorf(\"nil request in handler %d: %%w\", ErrNil)\n+\tresult, err := store.Fetch(ctx, req.ID, opts...)\n", h)
			b.WriteString(" \t}\n \tlogger.Info(\"ok\")\n \treturn nil\n")
			lines += 10
		}
	}
	return b.String()
}

func BenchmarkParse5000(b *testing.B) {
	raw := synthDiff(5000)
	if files, err := Parse(raw); err != nil || benchLines(files) < 4900 {
		b.Fatalf("synthetic diff did not parse: %d lines, %v", benchLines(files), err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	for i := 0; i < b.N; i++ {
		if _, err := Parse(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyWordDiff5000(b *testing.B) {
	raw := synthDiff(5000)
	files, _ := Parse(raw)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := range files {
			ApplyWordDiff(&files[j])
		}
	}
}

func BenchmarkParseWordDiff500(b *testing.B) {
	raw := synthDiff(500)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		files, _ := Parse(raw)
		for j := range files {
			ApplyWordDiff(&files[j])
		}
	}
}

func BenchmarkRows10000(b *testing.B) {
	// One file, 100 hunks of 100 lines (10,000 lines).
	var sb strings.Builder
	sb.WriteString("diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n")
	for h := 0; h < 100; h++ {
		start := 1 + h*150
		fmt.Fprintf(&sb, "@@ -%d,100 +%d,100 @@\n", start, start)
		for l := 0; l < 100; l++ {
			sb.WriteString(" context line of the file\n")
		}
	}
	files, _ := Parse(sb.String())
	f := &files[0]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rows := Rows(f, RowOptions{TotalNewLines: 15100})
		if len(rows) == 0 {
			b.Fatal("no rows")
		}
	}
}

func BenchmarkRelocate(b *testing.B) {
	content := make([]string, 10000)
	for i := range content {
		content[i] = fmt.Sprintf("line %d of the file", i%500)
	}
	a := NewAnchor("f.go", 5000, 5002, content)
	shifted := append([]string{"new"}, content...)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, ok := Relocate(a, shifted); !ok {
			b.Fatal("not found")
		}
	}
}

func benchLines(files []File) int {
	n := 0
	for i := range files {
		for _, h := range files[i].Hunks {
			n += len(h.Lines)
		}
	}
	return n
}
