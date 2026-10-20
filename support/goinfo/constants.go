package goinfo

const (
	GO_MAJOR_1    = 1
	GO_VERSION_27 = 27
	GO_VERSION_28 = 28
)

// The go1.N range xgo supports. script/drop-go-version raises
// MIN_SUPPORTED_MINOR when retiring a Go version.
const (
	MIN_SUPPORTED_MINOR = 27
	MAX_SUPPORTED_MINOR = 28
)
