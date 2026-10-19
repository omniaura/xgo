package main

import "fmt"

// VERSION, REVISION and NUMBER are set by `go run ./script/release`,
// which also tags the release; `go run ./script/release bump-core`
// moves REVISION and NUMBER together with the core ones below.
// They are no longer stamped on every commit.
const VERSION = "1.3.3"
const REVISION = "79112dd2d1f086d3475fefc95f936d73eba1af3f+1"
const NUMBER = 685

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
const CORE_REVISION = "bb465ba5d5e08c217c4c8f917b5b825dda5512c6+1"
const CORE_NUMBER = 683

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
