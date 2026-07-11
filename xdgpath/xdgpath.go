// Package xdgpath resolves per-application config/cache directories
// following the XDG Base Directory precedence, forced to ~/.config and
// ~/.cache on every platform (including macOS, which otherwise defaults to
// ~/Library/Application Support and ~/Library/Caches).
//
// Resolution order (most-specific first), for config:
//
//  1. $<APP>_CONFIG_DIR, if set and absolute — used verbatim as the app dir.
//  2. $XDG_CONFIG_HOME, if set and absolute — joined with app.
//  3. ~/.config, joined with app.
//
// Cache mirrors this with $<APP>_CACHE_DIR / $XDG_CACHE_HOME / ~/.cache.
// Per the XDG Base Directory spec, a $XDG_* value that is not an absolute
// path is treated as unset.
package xdgpath

import (
	"os"
	"path/filepath"

	"github.com/dangernoodle-io/mcpkit/internal/keyname"
)

// ConfigDir returns the resolved config directory for app.
func ConfigDir(app string) string {
	return resolve(app, "CONFIG_DIR", "XDG_CONFIG_HOME", ".config")
}

// CacheDir returns the resolved cache directory for app.
func CacheDir(app string) string {
	return resolve(app, "CACHE_DIR", "XDG_CACHE_HOME", ".cache")
}

// ConfigFile returns the path to name inside app's config directory.
func ConfigFile(app, name string) string {
	return filepath.Join(ConfigDir(app), name)
}

// CacheFile returns the path to name inside app's cache directory.
func CacheFile(app, name string) string {
	return filepath.Join(CacheDir(app), name)
}

// resolve implements the shared precedence for both ConfigDir and CacheDir.
// appSuffix is "CONFIG_DIR" or "CACHE_DIR"; xdgVar is the XDG_* env var
// name; homeSub is the fallback subdirectory of the user's home directory
// ("." + "config"/"cache").
func resolve(app, appSuffix, xdgVar, homeSub string) string {
	appEnv := envName(app) + "_" + appSuffix
	if v := os.Getenv(appEnv); v != "" && filepath.IsAbs(v) {
		return v
	}

	if v := os.Getenv(xdgVar); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, app)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	return filepath.Join(home, homeSub, app)
}

// envName upper-cases app and replaces every run of non-alphanumeric
// characters with a single underscore, for building <APP>_CONFIG_DIR /
// <APP>_CACHE_DIR env var names (e.g. "pogopin" -> "POGOPIN").
func envName(app string) string {
	return keyname.Upper(app)
}
