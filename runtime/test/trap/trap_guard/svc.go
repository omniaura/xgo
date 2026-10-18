// Package trap_guard checks the guarded trap prologue: instrumented
// functions skip the trap until something is registered, and pass the trap
// copies of their receiver, parameters and results. The shapes below follow
// common real-world usage: package-level client functions, methods on a
// service struct, generics, variadics and named results written by defers.
package trap_guard

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var initGoroutineDone = make(chan struct{})
var initGoroutineResult string

func init() {
	// goroutines started during init must not turn the trap on by
	// themselves (they share the init stack)
	go func() {
		initGoroutineResult = Charge(context.Background(), "init", 1)
		close(initGoroutineDone)
	}()
}

// Charge stands in for a call to an external payment API.
func Charge(ctx context.Context, customer string, cents int64) string {
	return fmt.Sprintf("charged %s %d", customer, cents)
}

type Store struct {
	prefix string
}

func (s *Store) Get(ctx context.Context, key string) (value string, err error) {
	if key == "" {
		return "", errors.New("empty key")
	}
	return s.prefix + key, nil
}

func (s Store) Name() string {
	return "store:" + s.prefix
}

func First[T any](items ...T) (first T, ok bool) {
	if len(items) == 0 {
		return first, false
	}
	return items[0], true
}

func Join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

// Normalize rewrites its named result in a defer, after the body returned.
func Normalize(s string) (out string) {
	defer func() {
		out = strings.ToUpper(out)
	}()
	return strings.TrimSpace(s)
}

// Increment changes its parameter before using it.
func Increment(n int) int {
	n++
	return n
}

func CallCharge(customer string) string {
	return Charge(context.Background(), customer, 100)
}

func CallGet(s *Store, key string) (string, error) {
	return s.Get(context.Background(), key)
}
