package main

import (
	"bytes"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCRAP(t *testing.T) {
	tests := []struct {
		c    int
		cov  float64
		want float64
	}{
		{15, 0, 240},
		{7, 0, 56},
		{15, 1, 15},
		{1, 0, 2},
		{4, 0.5, 6},
	}
	for _, tt := range tests {
		if got := CRAP(tt.c, tt.cov); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("CRAP(%d, %v) = %v, want %v", tt.c, tt.cov, got, tt.want)
		}
	}
}

const coverFunc = "github.com/ryanlewis/yat/main.go:37:\t\tmain\t\t0.0%\n" +
	"github.com/ryanlewis/yat/internal/item/parse.go:168:\t\tscan\t\t100.0%\n" +
	"github.com/ryanlewis/yat/internal/cmd/style.go:116:\t\tRenderBody\t\t50.0%\n" +
	"total:\t\t\t\t\t(statements)\t\t87.1%\n"

const cycloOut = "14 item (*headingScanner).scan internal/item/parse.go:168:1\n" +
	"15 main main main.go:37:1\n" +
	"6 cmd (Styles).RenderBody internal/cmd/style.go:116:1\n" +
	"3 cmd orphan internal/cmd/gone.go:9:1\n"

func TestRun_JoinsByFileLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cyclo.out")
	if err := os.WriteFile(path, []byte(cycloOut), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run(strings.NewReader(coverFunc), &out, &errOut, path, "github.com/ryanlewis/yat", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "skipped 1 functions") {
		t.Errorf("stderr = %q, want a skipped count for orphan", errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	want := [][]string{
		{"CRAP", "CC", "COV", "LOCATION", "FUNC"},
		{"240.0", "15", "0.0%", "main.go:37", "main"},
		{"14.0", "14", "100.0%", "internal/item/parse.go:168", "(*headingScanner).scan"},
		{"10.5", "6", "50.0%", "internal/cmd/style.go:116", "(Styles).RenderBody"},
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), out.String())
	}
	for i, w := range want {
		if got := strings.Fields(lines[i]); strings.Join(got, " ") != strings.Join(w, " ") {
			t.Errorf("line %d = %q, want %q", i, got, w)
		}
	}
}

func TestRun_Top(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cyclo.out")
	if err := os.WriteFile(path, []byte(cycloOut), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(strings.NewReader(coverFunc), &out, io.Discard, path, "github.com/ryanlewis/yat", 1); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "240.0") {
		t.Errorf("top 1 output:\n%s", out.String())
	}
}

func TestParseCyclo_BadLine(t *testing.T) {
	if _, _, err := parseCyclo(strings.NewReader("x main main main.go:1:1\n"), nil); err == nil {
		t.Error("want error for non-numeric complexity")
	}
	if _, _, err := parseCyclo(strings.NewReader("1 main main\n"), nil); err == nil {
		t.Error("want error for short line")
	}
	if _, _, err := parseCyclo(strings.NewReader("1 main main main.go\n"), nil); err == nil {
		t.Error("want error for a position with no column")
	}
}

func TestRun_NothingMatched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cyclo.out")
	if err := os.WriteFile(path, []byte(cycloOut), 0o600); err != nil {
		t.Fatal(err)
	}
	// Empty stdin is what a failing `go tool cover -func` leaves in the pipe.
	for name, tt := range map[string]struct{ cover, module string }{
		"empty cover input": {"", "github.com/ryanlewis/yat"},
		"wrong module":      {coverFunc, "example.com/other"},
		"no module":         {coverFunc, ""},
	} {
		t.Run(name, func(t *testing.T) {
			err := run(strings.NewReader(tt.cover), io.Discard, io.Discard, path, tt.module, 0)
			if err == nil {
				t.Error("want an error when no function has coverage data")
			}
		})
	}
}
