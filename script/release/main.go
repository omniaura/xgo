// Command release cuts a release of this fork, or bumps the core version.
//
//	go run ./script/release [flags] [patch|minor|major|X.Y.Z]
//	go run ./script/release bump-core
//
// A release sets VERSION, REVISION and NUMBER in cmd/xgo/version.go, commits
// that on master, tags the commit with the vX.Y.Z and runtime/vX.Y.Z pair,
// pushes the commit and both tags atomically, creates a GitHub release and
// asks the module proxy for both modules so consumers can fetch them at once.
//
// bump-core marks a change of the instrumentation or the runtime: it moves
// CORE_REVISION and CORE_NUMBER (and REVISION and NUMBER, which must not fall
// behind) to the current commit, then regenerates runtime/core/version.go and
// cmd/xgo/asset/runtime_gen. Commit the result with the change.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/omniaura/xgo/script/build-release/revision"
)

const usage = `usage:
  go run ./script/release [flags] [patch|minor|major|X.Y.Z]   (default: patch)
  go run ./script/release bump-core

flags:
`

func main() {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "print the steps without committing, tagging or pushing")
	noPush := fs.Bool("no-push", false, "commit and tag locally only")
	noGHRelease := fs.Bool("no-gh-release", false, "do not create a GitHub release")
	remote := fs.String("remote", "origin", "git remote to push to")
	branch := fs.String("branch", "master", "branch releases are cut from")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])
	if fs.NArg() > 1 {
		fs.Usage()
		os.Exit(2)
	}
	arg := fs.Arg(0)

	root, err := output("git", "rev-parse", "--show-toplevel")
	if err != nil {
		fatalf("%v", err)
	}
	if err := os.Chdir(root); err != nil {
		fatalf("%v", err)
	}

	if arg == "bump-core" {
		err = bumpCore()
	} else {
		err = release(arg, releaseOptions{
			dryRun:      *dryRun,
			noPush:      *noPush,
			noGHRelease: *noGHRelease,
			remote:      *remote,
			branch:      *branch,
		})
	}
	if err != nil {
		fatalf("%v", err)
	}
}

type releaseOptions struct {
	dryRun      bool
	noPush      bool
	noGHRelease bool
	remote      string
	branch      string
}

func release(bump string, opts releaseOptions) error {
	versionFile := revision.GetXgoVersionFile("")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return err
	}
	curVersion, _, curNumber, err := revision.ParseVersionConstants(versionFile, string(content))
	if err != nil {
		return err
	}
	version, err := nextVersion(curVersion, bump)
	if err != nil {
		return err
	}
	tag := "v" + version
	runtimeTag := "runtime/" + tag

	// preconditions: on the release branch, nothing uncommitted, and exactly
	// what the remote has, so the tags land on a commit others can see
	if cur, err := output("git", "rev-parse", "--abbrev-ref", "HEAD"); err != nil {
		return err
	} else if cur != opts.branch {
		return fmt.Errorf("on branch %s, releases are cut from %s", cur, opts.branch)
	}
	if dirty, err := output("git", "status", "--porcelain", "--untracked-files=no"); err != nil {
		return err
	} else if dirty != "" {
		return fmt.Errorf("uncommitted changes:\n%s", dirty)
	}
	if err := run("git", "fetch", "--quiet", "--tags", opts.remote, opts.branch); err != nil {
		return err
	}
	head, err := output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	remoteHead, err := output("git", "rev-parse", opts.remote+"/"+opts.branch)
	if err != nil {
		return err
	}
	if head != remoteHead {
		return fmt.Errorf("HEAD %s differs from %s/%s %s, pull or push first", head[:12], opts.remote, opts.branch, remoteHead[:12])
	}
	for _, t := range []string{tag, runtimeTag} {
		if _, err := output("git", "rev-parse", "-q", "--verify", "refs/tags/"+t); err == nil {
			return fmt.Errorf("tag %s already exists", t)
		}
	}

	fmt.Printf("release %s -> %s (NUMBER %d -> %d) at %s\n", curVersion, version, curNumber, curNumber+1, head[:12])
	if opts.dryRun {
		fmt.Printf("dry run: would commit %s, tag %s and %s, push to %s\n", versionFile, tag, runtimeTag, opts.remote)
		return nil
	}

	if err := revision.PatchVersionFile(versionFile, version, head+"+1", false, curNumber+1); err != nil {
		return err
	}
	if err := run("go", "build", "-o", os.DevNull, "./cmd/xgo"); err != nil {
		run("git", "checkout", "--", versionFile)
		return fmt.Errorf("building cmd/xgo: %w", err)
	}
	if err := run("git", "commit", "--quiet", "-m", "release "+tag, "--", versionFile); err != nil {
		run("git", "checkout", "--", versionFile)
		return err
	}
	for _, t := range []string{tag, runtimeTag} {
		if err := run("git", "tag", "-a", t, "-m", "xgo "+tag); err != nil {
			return err
		}
	}
	if opts.noPush {
		fmt.Printf("tagged %s and %s locally, push with: git push --atomic %s %s %s %s\n", tag, runtimeTag, opts.remote, opts.branch, tag, runtimeTag)
		return nil
	}
	if err := run("git", "push", "--quiet", "--atomic", opts.remote, opts.branch, tag, runtimeTag); err != nil {
		return err
	}

	repo, err := remoteRepo(opts.remote)
	if err != nil {
		fmt.Printf("released %s (%v: skipped the GitHub release and the module proxy)\n", tag, err)
		return nil
	}
	if !opts.noGHRelease {
		// -R: without it gh may resolve the upstream repo of a fork
		if err := run("gh", "release", "create", tag, "-R", repo, "--title", tag, "--generate-notes", "--verify-tag"); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: creating the GitHub release: %v\n", err)
		}
	}
	for _, mod := range []string{"github.com/" + repo, "github.com/" + repo + "/runtime"} {
		warmProxy(mod, tag)
	}
	fmt.Printf("released %s\n  go get github.com/%s@%s github.com/%s/runtime@%s\n", tag, repo, tag, repo, tag)
	return nil
}

// nextVersion applies bump (patch, minor, major or an explicit X.Y.Z) to cur.
func nextVersion(cur string, bump string) (string, error) {
	if bump == "" {
		bump = "patch"
	}
	explicit := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	if m := explicit.FindStringSubmatch(bump); m != nil {
		return m[1] + "." + m[2] + "." + m[3], nil
	}
	m := explicit.FindStringSubmatch(cur)
	if m == nil {
		return "", fmt.Errorf("current VERSION %q is not X.Y.Z, pass the version explicitly", cur)
	}
	var v [3]int
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	switch bump {
	case "major":
		v = [3]int{v[0] + 1, 0, 0}
	case "minor":
		v = [3]int{v[0], v[1] + 1, 0}
	case "patch":
		v[2]++
	default:
		return "", fmt.Errorf("unknown bump %q, want patch, minor, major or X.Y.Z", bump)
	}
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]), nil
}

// remoteRepo returns owner/name of the GitHub remote.
func remoteRepo(remote string) (string, error) {
	url, err := output("git", "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`github\.com[:/]([^/]+/[^/]+?)(\.git)?/?$`).FindStringSubmatch(url)
	if m == nil {
		return "", fmt.Errorf("remote %s is not a GitHub repository: %s", remote, url)
	}
	return m[1], nil
}

// warmProxy asks proxy.golang.org for mod@tag, so the first consumer does
// not wait for the proxy to fetch it. Failures are only reported.
func warmProxy(mod string, tag string) {
	url := "https://proxy.golang.org/" + strings.ToLower(mod) + "/@v/" + tag + ".info"
	client := &http.Client{Timeout: 60 * time.Second}
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := client.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("proxy: %s@%s %s\n", mod, tag, strings.TrimSpace(string(body)))
				return
			}
			err = fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		if attempt == 2 {
			fmt.Fprintf(os.Stderr, "WARNING: proxy %s@%s: %v\n", mod, tag, err)
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func bumpCore() error {
	versionFile := revision.GetXgoVersionFile("")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return err
	}
	_, _, number, err := revision.ParseVersionConstants(versionFile, string(content))
	if err != nil {
		return err
	}
	coreVersion, _, coreNumber, err := revision.ParseCoreVersionConstants(versionFile, string(content))
	if err != nil {
		return err
	}
	head, err := output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	rev := head + "+1"
	// the runtime warns when xgo's NUMBER is below its own, which is the
	// core one, so both move together
	num := number
	if coreNumber > num {
		num = coreNumber
	}
	num++

	code := string(content)
	for name, value := range map[string]string{
		"REVISION":      strconv.Quote(rev),
		"NUMBER":        strconv.Itoa(num),
		"CORE_REVISION": strconv.Quote(rev),
		"CORE_NUMBER":   strconv.Itoa(num),
	} {
		re := regexp.MustCompile(`(?m)^const ` + name + ` = .*$`)
		if !re.MatchString(code) {
			return fmt.Errorf("%s: missing const %s", versionFile, name)
		}
		code = re.ReplaceAllLiteralString(code, "const "+name+" = "+value)
	}
	if err := os.WriteFile(versionFile, []byte(code), 0644); err != nil {
		return err
	}
	if err := run("go", "run", "./script/generate", "--no-update-version", "runtime/core/version.go", "cmd/xgo/asset/runtime_gen"); err != nil {
		return err
	}
	fmt.Printf("core %s -> %s BUILD_%d (was BUILD_%d)\n", coreVersion, rev, num, coreNumber)
	return nil
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "release: "+format+"\n", args...)
	os.Exit(1)
}
