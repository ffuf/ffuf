package ffuf

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// used for random string generation in calibration function
var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString returns a random string of length of parameter n
func RandomString(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

// UniqStringSlice returns an unordered slice of unique strings. The duplicates are dropped
func UniqStringSlice(inslice []string) []string {
	found := map[string]bool{}

	for _, v := range inslice {
		found[v] = true
	}
	ret := []string{}
	for k := range found {
		ret = append(ret, k)
	}
	return ret
}

// FileExists checks if the filepath exists and is not a directory.
// Returns false in case it's not possible to describe the named file.
func FileExists(path string) bool {
	md, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !md.IsDir()
}

// RequestContainsKeyword checks if a keyword is present in any field of a request
func RequestContainsKeyword(req Request, kw string) bool {
	if strings.Contains(req.Host, kw) {
		return true
	}
	if strings.Contains(req.Url, kw) {
		return true
	}
	if strings.Contains(req.Method, kw) {
		return true
	}
	if strings.Contains(string(req.Data), kw) {
		return true
	}
	for k, v := range req.Headers {
		if strings.Contains(k, kw) || strings.Contains(v, kw) {
			return true
		}
	}
	return false
}

// HostURLFromRequest gets a host + path without the filename or last part of the URL path
func HostURLFromRequest(req Request) string {
	u, _ := url.Parse(req.Url)
	u.Host = req.Host
	pathparts := strings.Split(u.Path, "/")
	trimpath := strings.TrimSpace(strings.Join(pathparts[:len(pathparts)-1], "/"))
	return u.Host + trimpath
}

// Version returns the ffuf version string.
//
// It resolves through four sources, in this order:
//
//  1. Release builds have VERSION injected via -ldflags by goreleaser and
//     VERSION_APPENDIX emptied, so they report a plain semantic version like
//     "2.2.0" with no manual constant bump required.
//  2. Builds from a source checkout report a "git-<UTC date>-<short commit>"
//     identifier derived from the VCS metadata `go build` embeds, e.g.
//     "git-20260613-aabbccdd". VCS metadata takes precedence over the module
//     version below, because a working tree can sit on a tagged commit while
//     carrying uncommitted changes.
//  3. Binaries produced by `go install github.com/ffuf/ffuf/v2@vX.Y.Z` carry no
//     VCS metadata, but the toolchain records the module version they were built
//     from, so report that.
//  4. Failing all of those, VERSION+VERSION_APPENDIX, which is a placeholder
//     rather than a real version.
func Version() string {
	v, _ := resolveVersion()
	return v
}

var (
	versionOnce     sync.Once
	resolvedVersion string
	versionReleased bool
)

// resolveVersion resolves the version once and caches it. Version() sits on the
// per-request path since it fills in the default User-Agent, and both metadata
// lookups below parse data embedded in the binary, so resolving on every call
// spends microseconds per request on a value that cannot change.
func resolveVersion() (string, bool) {
	versionOnce.Do(func() {
		resolvedVersion, versionReleased = selectVersion(VERSION, VERSION_APPENDIX, gitVersion(), moduleVersion())
	})
	return resolvedVersion, versionReleased
}

// selectVersion picks which of the available version sources to report, and
// reports whether the result identifies a released version.
//
// VCS metadata deliberately outranks the module version: a working tree can sit
// on a tagged commit while carrying uncommitted changes, and the toolchain
// records the tag as the module version regardless, so preferring it would let a
// dirty build claim to be a clean release.
func selectVersion(injected, appendix, git, module string) (version string, released bool) {
	if appendix == "" {
		return injected, true
	}
	if git != "" {
		return git, false
	}
	if module != "" {
		return module, true
	}
	return injected + appendix, false
}

// moduleVersion returns the version of the main module this binary was built
// from, as recorded by `go install module@version`. It returns "" when there is
// no real version to report, which is the case for a plain `go build` in a
// working tree.
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return normalizeModuleVersion(info.Main.Version)
}

// normalizeModuleVersion strips the module version's leading "v" and rejects the
// placeholders the toolchain uses when no version is available.
func normalizeModuleVersion(v string) string {
	if v == "" || v == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}

// gitVersion assembles a "git-<date>-<shorthash>" string from the VCS metadata
// that `go build` stamps into the binary. It returns "" when the metadata is
// missing (e.g. a module build outside of a repository).
func gitVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var revision string
	var modified bool
	var stamp time.Time
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			stamp, _ = time.Parse(time.RFC3339, s.Value)
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" || stamp.IsZero() {
		return ""
	}
	if len(revision) > 8 {
		revision = revision[:8]
	}
	dirty := ""
	if modified {
		dirty = "-dirty"
	}
	return fmt.Sprintf("git-%s-%s%s", stamp.UTC().Format("20060102"), revision, dirty)
}

// FormattedVersion returns the version prepared for display. Released versions
// are prefixed with "v" (e.g. "v2.2.0"); development builds are returned
// unprefixed (e.g. "git-20260613-aabbccdd") since a "v" reads as noise there.
func FormattedVersion() string {
	v, released := resolveVersion()
	if released {
		return "v" + v
	}
	return v
}

func CheckOrCreateConfigDir() error {
	var err error
	err = CreateConfigDir(CONFIGDIR)
	if err != nil {
		return err
	}
	err = CreateConfigDir(HISTORYDIR)
	if err != nil {
		return err
	}
	err = CreateConfigDir(SCRAPERDIR)
	if err != nil {
		return err
	}
	err = CreateConfigDir(AUTOCALIBDIR)
	if err != nil {
		return err
	}
	err = setupDefaultAutocalibrationStrategies()
	return err
}

func CreateConfigDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		var pError *os.PathError
		if errors.As(err, &pError) {
			return os.MkdirAll(path, 0750)
		}
		return err
	}
	return nil
}

func StrInSlice(key string, slice []string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}
	return false
}
