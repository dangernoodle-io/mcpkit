// Package keyname provides the shared key-derivation transform used by
// xdgpath (app-name -> env var prefix) and store/env (store key -> env var
// suffix): uppercase, with every run of non-alphanumeric characters
// collapsed to a single underscore.
package keyname

import (
	"strings"
	"unicode"
)

// Upper uppercases s and collapses every run of non-alphanumeric
// characters to a single underscore (e.g. "db.path" -> "DB_PATH",
// "my-cool-app" -> "MY_COOL_APP").
func Upper(s string) string {
	var b strings.Builder

	prevUnderscore := false

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
			prevUnderscore = false

			continue
		}

		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}

	return b.String()
}
