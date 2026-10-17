// Package deps: chapter 08 of docs/go-map. Using a third-party module
// (github.com/google/uuid) and inspecting the dependencies of a binary.
package deps

import (
	"runtime/debug"
	"sort"

	"github.com/google/uuid"
)

// NewPaymentID uses a dependency declared in go.mod. Run `go mod why github.com/google/uuid`
// to see which packages need it.
func NewPaymentID() string {
	return "txn_" + uuid.Must(uuid.NewV7()).String()
}

// GoVersion returns the toolchain version that built the running binary.
func GoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.GoVersion
}

// Dependencies lists the modules compiled into the running binary, read from
// the build info Go embeds in every executable (same data as `go version -m <binary>`).
// Test binaries do not record dependencies, so this is empty under `go test`;
// try it on a real binary: go build -o /tmp/api ./cmd/api && go version -m /tmp/api
func Dependencies() []string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	var out []string
	for _, d := range info.Deps {
		out = append(out, d.Path+"@"+d.Version)
	}
	sort.Strings(out)
	return out
}
