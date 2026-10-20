// drop-go-version retires every Go version older than go1.<min>.
//
// usage:
//
//	go run ./script/drop-go-version 1.28          # apply
//	go run ./script/drop-go-version --dry-run 1.28
//
// It does the mechanical part:
//   - deletes patches/go1.N and cmd/xgo/asset/patches/go1.N for N < min
//   - deletes .github/workflows/go1-N*.yml for N < min and moves the remaining
//     workflows' go-version pins to 1.<min>
//   - raises goinfo.MIN_SUPPORTED_MINOR
//   - folds //go:build go1.N constraints (N <= min is now always true) in the
//     root, patch and runtime/test modules: files that can no longer build are
//     deleted, constraints that became always true are removed. A go1.N tag
//     newer than its module's go directive also raises that file's language
//     version, so it is kept there.
//   - regenerates the embedded assets
//
// Then it lists the remaining go1.N mentions and Minor comparisons below min
// for review. Code that only served the retired versions usually shows up in
// `deadcode -test` afterwards.
//
// go directives are never raised: user modules import runtime, and fixture
// modules import runtime/test, so a newer directive there would force them to
// update their own go.mod. The runtime module and the nested fixture modules
// under runtime/test/** are not folded at all.
package main

import (
	"fmt"
	"go/build/constraint"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// modules whose build constraints are folded, relative to the repo root
var foldModules = []string{".", "patch", "runtime/test"}

// directories never touched by folding
var skipDirs = []string{".git", "cmd/xgo/asset", "cmd/xgo/internal/vendir", "patches", "doc"}

func main() {
	var dryRun bool
	var version string
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "-"):
			exit(fmt.Errorf("unrecognized flag: %s", arg))
		default:
			version = arg
		}
	}
	min, err := parseMinor(version)
	if err != nil {
		exit(err)
	}
	d := &dropper{min: min, dryRun: dryRun}
	steps := []func() error{d.patches, d.workflows, d.minConst, d.fold}
	if !dryRun {
		steps = append(steps, d.generate)
	}
	for _, step := range steps {
		if err := step(); err != nil {
			exit(err)
		}
	}
	d.report()
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

func parseMinor(version string) (int, error) {
	v := strings.TrimPrefix(version, "go")
	if !strings.HasPrefix(v, "1.") {
		return 0, fmt.Errorf("usage: drop-go-version [--dry-run] 1.N")
	}
	n, err := strconv.Atoi(v[2:])
	if err != nil {
		return 0, fmt.Errorf("bad version %q: %w", version, err)
	}
	return n, nil
}

type dropper struct {
	min    int
	dryRun bool
}

func (d *dropper) logf(format string, args ...interface{}) {
	prefix := ""
	if d.dryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf(prefix+format+"\n", args...)
}

func (d *dropper) remove(path string) error {
	d.logf("delete %s", path)
	if d.dryRun {
		return nil
	}
	return os.RemoveAll(path)
}

func (d *dropper) write(path string, content string) error {
	d.logf("update %s", path)
	if d.dryRun {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

var goDirRe = regexp.MustCompile(`^go1\.(\d+)$`)

func (d *dropper) patches() error {
	for _, dir := range []string{"patches", filepath.Join("cmd", "xgo", "asset", "patches")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			m := goDirRe.FindStringSubmatch(e.Name())
			if m == nil || !e.IsDir() {
				continue
			}
			if n, _ := strconv.Atoi(m[1]); n < d.min {
				if err := d.remove(filepath.Join(dir, e.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var (
	workflowFileRe = regexp.MustCompile(`^go1-(\d+)(-.*)?\.ya?ml$`)
	goVersionPinRe = regexp.MustCompile(`(go-version: '|go: \[ ')1\.(\d+)'`)
)

func (d *dropper) workflows() error {
	dir := filepath.Join(".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if m := workflowFileRe.FindStringSubmatch(e.Name()); m != nil {
			if n, _ := strconv.Atoi(m[1]); n < d.min {
				if err := d.remove(path); err != nil {
					return err
				}
				continue
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := goVersionPinRe.ReplaceAllStringFunc(string(data), func(s string) string {
			m := goVersionPinRe.FindStringSubmatch(s)
			if n, _ := strconv.Atoi(m[2]); n < d.min {
				return fmt.Sprintf("%s1.%d'", m[1], d.min)
			}
			return s
		})
		if content != string(data) {
			if err := d.write(path, content); err != nil {
				return err
			}
		}
	}
	return nil
}

var minConstRe = regexp.MustCompile(`MIN_SUPPORTED_MINOR = (\d+)`)

func (d *dropper) minConst() error {
	path := filepath.Join("support", "goinfo", "constants.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m := minConstRe.FindSubmatch(data)
	if m == nil {
		return fmt.Errorf("%s: MIN_SUPPORTED_MINOR not found", path)
	}
	if cur, _ := strconv.Atoi(string(m[1])); cur >= d.min {
		return nil
	}
	return d.write(path, minConstRe.ReplaceAllString(string(data), fmt.Sprintf("MIN_SUPPORTED_MINOR = %d", d.min)))
}

func (d *dropper) fold() error {
	for _, mod := range foldModules {
		langMinor, err := goDirectiveMinor(filepath.Join(mod, "go.mod"))
		if err != nil {
			return err
		}
		err = filepath.WalkDir(mod, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				for _, skip := range skipDirs {
					if path == skip {
						return filepath.SkipDir
					}
				}
				// nested modules are folded only when listed
				if path != mod && fileExists(filepath.Join(path, "go.mod")) && !contains(foldModules, path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			return d.foldFile(path, langMinor)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dropper) foldFile(path string, langMinor int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content, keep, changed, err := foldConstraints(string(data), d.min, langMinor)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !keep {
		return d.remove(path)
	}
	if changed {
		return d.write(path, content)
	}
	return nil
}

// foldConstraints rewrites the //go:build line of src for a minimum go1.<min>
// in a module whose go directive is go1.<langMinor>. keep is false when the
// file can no longer build.
func foldConstraints(src string, min int, langMinor int) (content string, keep bool, changed bool, err error) {
	lines := strings.SplitAfter(src, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if strings.HasPrefix(line, "package ") {
			out = append(out, lines[i:]...)
			break
		}
		if constraint.IsPlusBuild(line) {
			// obsolete since go1.17; the //go:build line is authoritative
			changed = true
			continue
		}
		if !constraint.IsGoBuild(line) {
			out = append(out, lines[i])
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return "", false, false, err
		}
		if _, lit := fold(expr, min); lit == litFalse {
			return "", false, true, nil
		}
		// only tags the module's language version already implies can go
		folded, lit := fold(expr, minInt(min, langMinor))
		switch lit {
		case litFalse:
			// unreachable: a weaker assumption cannot make it false
			return "", false, true, nil
		case litTrue:
			changed = true
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
				i++
			}
		default:
			if folded.String() != expr.String() {
				changed = true
			}
			out = append(out, "//go:build "+folded.String()+"\n")
		}
	}
	return strings.Join(out, ""), true, changed, nil
}

type literal int

const (
	litUnknown literal = iota
	litTrue
	litFalse
)

// fold simplifies e assuming go1.N holds for every N <= min.
func fold(e constraint.Expr, min int) (constraint.Expr, literal) {
	switch x := e.(type) {
	case *constraint.TagExpr:
		if n, ok := goTag(x.Tag); ok && n <= min {
			return nil, litTrue
		}
		return x, litUnknown
	case *constraint.NotExpr:
		in, l := fold(x.X, min)
		switch l {
		case litTrue:
			return nil, litFalse
		case litFalse:
			return nil, litTrue
		}
		return &constraint.NotExpr{X: in}, litUnknown
	case *constraint.AndExpr:
		a, la := fold(x.X, min)
		b, lb := fold(x.Y, min)
		switch {
		case la == litFalse || lb == litFalse:
			return nil, litFalse
		case la == litTrue:
			return b, lb
		case lb == litTrue:
			return a, la
		}
		return &constraint.AndExpr{X: a, Y: b}, litUnknown
	case *constraint.OrExpr:
		a, la := fold(x.X, min)
		b, lb := fold(x.Y, min)
		switch {
		case la == litTrue || lb == litTrue:
			return nil, litTrue
		case la == litFalse:
			return b, lb
		case lb == litFalse:
			return a, la
		}
		return &constraint.OrExpr{X: a, Y: b}, litUnknown
	}
	panic(fmt.Sprintf("unexpected constraint %T", e))
}

func goTag(tag string) (int, bool) {
	if !strings.HasPrefix(tag, "go1.") {
		return 0, false
	}
	n, err := strconv.Atoi(tag[len("go1."):])
	return n, err == nil
}

func (d *dropper) generate() error {
	d.logf("regenerate embedded assets")
	cmd := exec.Command("go", "run", "./script/generate",
		"patch/instrument", "cmd/xgo/asset/compiler_patch_gen", "cmd/xgo/asset/patches", "cmd/xgo/asset/runtime_gen")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// report lists what is left for a human to review.
func (d *dropper) report() {
	var pats []string
	for n := 1; n < d.min; n++ {
		pats = append(pats, strconv.Itoa(n))
	}
	alt := strings.Join(pats, "|")
	re := regexp.MustCompile(`GO_VERSION_(` + alt + `)\b|Minor *(<|<=|==|>=|>|!=) *(` + alt + `)\b`)
	var hits []string
	filepath.WalkDir(".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			for _, skip := range append(skipDirs, "runtime/test") {
				if path == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	sort.Strings(hits)
	fmt.Printf("\nversion checks below go1.%d left for review (%d):\n", d.min, len(hits))
	for _, h := range hits {
		fmt.Println("  " + h)
	}
	fmt.Println("\nnext: review the list, run `deadcode -test` over the module, then `go run ./script/release bump-core` and the tests")
}

var goDirectiveRe = regexp.MustCompile(`(?m)^go 1\.(\d+)`)

func goDirectiveMinor(goMod string) (int, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return 0, err
	}
	m := goDirectiveRe.FindSubmatch(data)
	if m == nil {
		return 0, fmt.Errorf("%s: no go directive", goMod)
	}
	return strconv.Atoi(string(m[1]))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
