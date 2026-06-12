package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/citylayout"
)

type cityDiscoveryOptions struct {
	ceilingDirs          []string
	ignoredLegacyRuntime []string
}

// findCity walks dir upward looking for a directory containing city.toml.
// Implicit discovery is bounded so it does not accidentally resolve unrelated
// ancestors such as $HOME or the supervisor's global ~/.gc runtime root.
func findCity(dir string) (string, error) {
	return findCityWithOptions(dir, implicitCityDiscoveryOptions())
}

func findCityWithOptions(dir string, opts cityDiscoveryOptions) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	var legacy string
	for {
		if citylayout.HasCityConfig(dir) {
			return dir, nil
		}
		if legacy == "" && citylayout.HasRuntimeRoot(dir) && !isIgnoredLegacyRuntimeRoot(dir, opts.ignoredLegacyRuntime) {
			legacy = dir
		}
		if isCityDiscoveryCeiling(dir, opts.ceilingDirs) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if legacy != "" {
		return legacy, nil
	}
	return "", fmt.Errorf("not in a city directory (no city.toml or .gc/ found)")
}

func implicitCityDiscoveryOptions() cityDiscoveryOptions {
	return cityDiscoveryOptions{
		ceilingDirs:          implicitCityDiscoveryCeilings(),
		ignoredLegacyRuntime: implicitIgnoredLegacyRuntimeRoots(),
	}
}

func implicitCityDiscoveryCeilings() []string {
	if raw := strings.TrimSpace(os.Getenv("GC_CEILING_DIRECTORIES")); raw != "" {
		return normalizeDiscoveryPaths(strings.Split(raw, string(os.PathListSeparator)))
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	return normalizeDiscoveryPaths([]string{home})
}

func implicitIgnoredLegacyRuntimeRoots() []string {
	var roots []string
	if runtimeRoot := configuredSupervisorRuntimeRoot(); runtimeRoot != "" {
		roots = append(roots, runtimeRoot)
	}
	// Also ignore .gc/ at any OS system temp directory. A supervisor or system
	// test may create runtime state at /tmp/.gc; that must not be discovered
	// as a city root when walking upward from a path under /tmp.
	// Note: TMPDIR may be overridden by test infrastructure to a subdirectory
	// of /tmp, so we always include the well-known /tmp root on Unix in
	// addition to os.TempDir().
	for _, sysTemp := range implicitSystemTempRoots() {
		roots = append(roots, filepath.Join(sysTemp, citylayout.RuntimeRoot))
	}
	return roots
}

// implicitSystemTempRoots returns the set of OS-level system temporary
// directories that should not be treated as Gas City project roots.
// TMPDIR may be overridden (e.g. by test infrastructure) to a path under
// the real /tmp, so we always include /tmp explicitly on Unix.
func implicitSystemTempRoots() []string {
	seen := make(map[string]bool)
	var roots []string
	for _, d := range []string{os.TempDir(), "/tmp"} {
		d = filepath.Clean(d)
		if d == "" || d == "." || seen[d] {
			continue
		}
		seen[d] = true
		roots = append(roots, d)
	}
	return roots
}

func configuredSupervisorRuntimeRoot() string {
	if gcHome := strings.TrimSpace(os.Getenv("GC_HOME")); gcHome != "" {
		return normalizeDiscoveryPath(gcHome)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(normalizeDiscoveryPath(home), citylayout.RuntimeRoot)
}

func isCityDiscoveryCeiling(dir string, ceilings []string) bool {
	dir = normalizeDiscoveryPath(dir)
	for _, ceiling := range ceilings {
		if dir == ceiling {
			return true
		}
	}
	return false
}

func isIgnoredLegacyRuntimeRoot(dir string, ignored []string) bool {
	runtimeRoot := filepath.Join(normalizeDiscoveryPath(dir), citylayout.RuntimeRoot)
	for _, candidate := range ignored {
		if runtimeRoot == candidate {
			return true
		}
	}
	return false
}

func normalizeDiscoveryPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = normalizeDiscoveryPath(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func normalizeDiscoveryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}
