package main

import "fmt"

// VERSION, REVISION and NUMBER are set by `go run ./script/release`,
// which also tags the release; `go run ./script/release bump-core`
// moves REVISION and NUMBER together with the core ones below.
// They are no longer stamped on every commit.
const VERSION = "1.3.0"
const REVISION = "768ba6d2c53c3c0dbfd8ab61bba30ee17bde1e35+1"
const NUMBER = 681

// Rationale: xgo consists of these modules:
//
// - cmd/xgo
// - cmd/xgo/test-explorer
// - cmd/xgo/trace
// - runtime
// - cmd/xgo/runtime_gen (generated from runtime)
//
// They share the same version, but not every commit
// affects the instrumentation.
//
// To distinguish them,
// we use CORE_REVISION and CORE_NUMBER.
// The core version denotes the core functionality
// of:
//   - cmd/xgo
//   - runtime
//
// general rule is, if you changed anything that
// affects cmd/xgo or runtime, bump the CORE_REVISION
// and CORE_NUMBER.
//
// The CORE_VERSION is manually updated
// once updated, it will be copied to runtime/core/version.go
//
// to update, run `go run ./script/release bump-core`, it sets both groups
// of constants to the current commit and regenerates the runtime copies.
const CORE_VERSION = "1.2.2"
const CORE_REVISION = "04ac83543bcc3f8a76e32472c44c7d39e283052d+1"
const CORE_NUMBER = 650

func getRevision() string {
	return formatRevision(VERSION, REVISION, NUMBER)
}

func getCoreRevision() string {
	return formatRevision(CORE_VERSION, CORE_REVISION, CORE_NUMBER)
}

func formatRevision(version string, revision string, number int) string {
	revSuffix := ""
	if isDevelopment {
		revSuffix = "_DEV"
	}
	return fmt.Sprintf("%s %s%s BUILD_%d", version, revision, revSuffix, number)
}
