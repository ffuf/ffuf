package ffuf

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
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
// Release builds have their version injected via -ldflags by goreleaser: VERSION
// is set to the git tag and VERSION_APPENDIX is emptied, so they report a plain
// semantic version like "2.2.0" with no manual constant bump required.
//
// Builds from a source checkout (VERSION_APPENDIX still set) instead report a
// "git-<UTC date>-<short commit>" identifier derived from the VCS metadata that
// `go build` embeds into the binary, e.g. "git-20260613-aabbccdd". When that
// metadata is unavailable it falls back to VERSION+VERSION_APPENDIX.
func Version() string {
	if VERSION_APPENDIX == "" {
		return VERSION
	}
	if v := gitVersion(); v != "" {
		return v
	}
	return fmt.Sprintf("%s%s", VERSION, VERSION_APPENDIX)
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

// FormattedVersion returns the version prepared for display. Release builds are
// prefixed with "v" (e.g. "v2.2.0"); development builds are returned unprefixed
// (e.g. "git-20260613-aabbccdd") since a "v" reads as noise there.
func FormattedVersion() string {
	if VERSION_APPENDIX == "" {
		return "v" + Version()
	}
	return Version()
}

func CheckOrCreateConfigDir() error {
	var err error
	err = createConfigDir(CONFIGDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(HISTORYDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(SCRAPERDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(AUTOCALIBDIR)
	if err != nil {
		return err
	}
	err = setupDefaultAutocalibrationStrategies()
	return err
}

func createConfigDir(path string) error {
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

func mergeMaps(m1 map[string][]string, m2 map[string][]string) map[string][]string {
	merged := make(map[string][]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		if _, ok := merged[key]; !ok {
			// Key not found, add it
			merged[key] = value
			continue
		}
		for _, entry := range value {
			if !StrInSlice(entry, merged[key]) {
				merged[key] = append(merged[key], entry)
			}
		}
	}
	return merged
}
