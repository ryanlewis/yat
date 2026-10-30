// Command crap joins gocyclo output with `go tool cover -func` output and
// prints the functions with the highest CRAP score.
//
// CRAP = c² × (1 − cov)³ + c, where c is the cyclomatic complexity and cov
// is the function's statement coverage as a fraction.
//
// Usage:
//
//	go tool cover -func=coverage.out | go run ./internal/tools/crap -cyclo cyclo.out -module <path> -top 20
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

const (
	defaultTop  = 20
	percent     = 100
	cycloFields = 4 // fields in a gocyclo line: complexity, package, function, file:line:col
)

// Func is one function with its complexity, coverage and CRAP score.
type Func struct {
	Complexity int
	Coverage   float64 // fraction, 0 to 1
	Location   string  // file:line, relative to the module root
	Name       string
}

// CRAP returns the CRAP score for complexity c and coverage fraction cov.
func CRAP(c int, cov float64) float64 {
	cc := float64(c)
	u := 1 - cov
	return cc*cc*u*u*u + cc
}

// Score returns the function's CRAP score.
func (f Func) Score() float64 { return CRAP(f.Complexity, f.Coverage) }

// parseCover reads `go tool cover -func` output into a map from file:line
// (with the module prefix removed) to coverage fraction.
func parseCover(r io.Reader, module string) (map[string]float64, error) {
	prefix := module + "/"
	cov := map[string]float64{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 || fields[0] == "total:" {
			continue
		}
		loc := strings.TrimSuffix(strings.TrimPrefix(fields[0], prefix), ":")
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		if err != nil {
			return nil, fmt.Errorf("cover line %q: %w", sc.Text(), err)
		}
		cov[loc] = pct / percent
	}
	return cov, sc.Err()
}

// parseCyclo reads gocyclo output and joins each function with its coverage.
// Functions with no coverage entry are left out and counted in skipped: they
// sit in files not built for this platform (build tags), so have no score.
func parseCyclo(r io.Reader, cov map[string]float64) (funcs []Func, skipped int, err error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != cycloFields {
			return nil, 0, fmt.Errorf("gocyclo line %q: want %d fields", sc.Text(), cycloFields)
		}
		c, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, 0, fmt.Errorf("gocyclo line %q: %w", sc.Text(), err)
		}
		pos := fields[3]
		col := strings.LastIndex(pos, ":")
		if col < 0 {
			return nil, 0, fmt.Errorf("gocyclo line %q: no file:line:col", sc.Text())
		}
		loc := pos[:col]
		fc, ok := cov[loc]
		if !ok {
			skipped++
			continue
		}
		funcs = append(funcs, Func{Complexity: c, Coverage: fc, Location: loc, Name: fields[2]})
	}
	return funcs, skipped, sc.Err()
}

// rank sorts funcs by CRAP score, highest first, breaking ties by location.
func rank(funcs []Func) {
	sort.Slice(funcs, func(i, j int) bool {
		si, sj := funcs[i].Score(), funcs[j].Score()
		if si != sj {
			return si > sj
		}
		return funcs[i].Location < funcs[j].Location
	})
}

func write(w io.Writer, funcs []Func) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CRAP\tCC\tCOV\tLOCATION\tFUNC")
	for _, f := range funcs {
		fmt.Fprintf(tw, "%.1f\t%d\t%.1f%%\t%s\t%s\n", f.Score(), f.Complexity, f.Coverage*percent, f.Location, f.Name)
	}
	return tw.Flush()
}

func run(stdin io.Reader, stdout, stderr io.Writer, cycloPath, module string, top int) error {
	if module == "" {
		return errors.New("-module is required")
	}
	cov, err := parseCover(stdin, module)
	if err != nil {
		return err
	}
	file, err := os.Open(cycloPath)
	if err != nil {
		return err
	}
	defer file.Close()
	funcs, skipped, err := parseCyclo(file, cov)
	if err != nil {
		return err
	}
	if len(funcs) == 0 && skipped > 0 {
		return fmt.Errorf("none of %d functions have coverage data; check -module and the cover input", skipped)
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "crap: skipped %d functions with no coverage data (not built on this platform)\n", skipped)
	}
	rank(funcs)
	if top > 0 && top < len(funcs) {
		funcs = funcs[:top]
	}
	return write(stdout, funcs)
}

func main() {
	cycloPath := flag.String("cyclo", "", "file with `go tool gocyclo -over 0` output")
	module := flag.String("module", "", "module path to strip from cover -func locations")
	top := flag.Int("top", defaultTop, "number of functions to print (0 for all)")
	flag.Parse()
	if err := run(os.Stdin, os.Stdout, os.Stderr, *cycloPath, *module, *top); err != nil {
		fmt.Fprintln(os.Stderr, "crap:", err)
		os.Exit(1)
	}
}
