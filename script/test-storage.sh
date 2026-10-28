#!/usr/bin/env bash
set -euo pipefail

# Every writable path belongs to this test. Never pressure-test the user's
# real cache or delete global legacy Go temporary directories.
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT
mkdir -p "$test_root/home" "$test_root/project"
go build -o "$test_root/xgo" ./cmd/xgo
cat > "$test_root/project/go.mod" <<'EOF'
module example.com/xgo-storage-smoke

go 1.27.0
EOF
cat > "$test_root/project/main.go" <<'EOF'
package main
import ("fmt"; "os")
func main() { fmt.Print(os.Args[1]) }
EOF
cat > "$test_root/project/main_test.go" <<'EOF'
package main
import ("flag"; "os"; "strings"; "testing")
var programArgument = flag.String("cache-max-size", "", "test passthrough flag")
func TestManagedTemp(t *testing.T) {
    if !strings.Contains(os.TempDir(), "sessions-") { t.Fatal(os.TempDir()) }
    if *programArgument != "program-argument" { t.Fatal(os.Args) }
}
func TestPure(t *testing.T) { if 2+2 != 4 { t.Fatal("arithmetic") } }
EOF
cat > "$test_root/project/.xgo.yaml" <<'EOF'
cache-dir: ../cache
cache-max-size: 20GiB
cache-min-free: 1GiB
EOF
export GOFLAGS=-trimpath XGO_FLAGS=""
export XGO_BUILD_CACHE_DIR="$test_root/shared-build-cache"
"$test_root/xgo" --help > "$test_root/help"
grep -q 'completion' "$test_root/help"
"$test_root/xgo" test --xgo-home "$test_root/home" --project-dir "$test_root/project" -v ./... -args --cache-max-size=program-argument
"$test_root/xgo" test --xgo-home "$test_root/home" --project-dir "$test_root/project" -run TestPure -v ./... > "$test_root/warm-seed"
"$test_root/xgo" test --xgo-home "$test_root/home" --project-dir "$test_root/project" -run TestPure -v ./... > "$test_root/warm"
grep -q '(cached)' "$test_root/warm"
"$test_root/xgo" run --xgo-home "$test_root/home" --project-dir "$test_root/project" . --cache-max-size=program-argument > "$test_root/run"
grep -q -- '--cache-max-size=program-argument' "$test_root/run"
"$test_root/xgo" exec --xgo-home "$test_root/home" --project-dir "$test_root/project" go version
printf 'stdin-ok\n' | "$test_root/xgo" exec --xgo-home "$test_root/home" --project-dir "$test_root/project" sh -c 'read value; printf "%s" "$value"' > "$test_root/stdin"
grep -q 'stdin-ok' "$test_root/stdin"
"$test_root/xgo" cache status --project-dir "$test_root/project" --xgo-home "$test_root/home" > "$test_root/status"
# Explicitly impossible reserve: must fail before starting the child.
if "$test_root/xgo" exec --xgo-home "$test_root/home" --project-dir "$test_root/project" --cache-min-free=9223372036854775807 sh -c 'exit 42' > "$test_root/refusal" 2>&1; then
    echo 'disk reserve did not refuse the command' >&2
    exit 1
fi
grep -q 'low disk space' "$test_root/refusal"
# A completed build must not leave a session containing compiler/test data.
if find "$test_root/cache" -name 'run-*' -type d | grep -q .; then
    echo 'completed build left a temporary session' >&2
    exit 1
fi
echo 'storage smoke passed: cold/warm test, run args, exec/stdin, reserve refusal, session cleanup'
