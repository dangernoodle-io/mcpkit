// Package identity resolves a session identity by trying an ordered chain
// of sources, first non-empty wins. It is host-agnostic: it never imports a
// host-specific payload/hooks type — a caller that has an out-of-band value
// (e.g. a decoded stdin session_id) splices it in via Static. The terminal
// fallback is opt-in: a chain with no UUID source returns "" when nothing
// matches, so the package never hard-depends on any host session env var.
package identity

import (
	"crypto/rand"
	"fmt"
	"os"
)

// Source returns "" to mean "no opinion, keep looking".
type Source func() string

// Env returns a Source reading environment variable name. Serves both the
// override tier (a consumer-chosen var) and the host-env-probe tier (e.g.
// CLAUDE_CODE_SESSION_ID) — same primitive, position in the chain decides role.
func Env(name string) Source {
	return func() string {
		return os.Getenv(name)
	}
}

// Static returns a Source that always yields id — how a caller injects a
// value it already decoded (a statusline Payload / hooks Common session_id)
// without this package importing that host-specific type.
func Static(id string) Source {
	return func() string {
		return id
	}
}

// UUID returns a Source that always yields a fresh NewV4 UUID. A terminal
// fallback, wired in only if the caller appends it (opt-in).
func UUID() Source {
	return func() string {
		return NewV4()
	}
}

// UUIDFunc returns a Source that always yields gen() — for injecting a
// deterministic generator in tests.
func UUIDFunc(gen func() string) Source {
	return func() string {
		return gen()
	}
}

// NewV4 returns a random RFC 4122 version-4 UUID via crypto/rand (no dep).
// If crypto/rand.Read fails, unfilled bytes stay zero, yielding a well-formed but low-entropy UUID rather than panicking.
// NewV4 never panics or returns a malformed string.
func NewV4() string {
	var b [16]byte

	_, _ = rand.Read(b[:])

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Resolve evaluates sources in order, returning the first non-empty result,
// or "" if all return "" (or no sources given). A nil Source is skipped.
func Resolve(sources ...Source) string {
	for _, src := range sources {
		if src == nil {
			continue
		}

		if v := src(); v != "" {
			return v
		}
	}

	return ""
}
