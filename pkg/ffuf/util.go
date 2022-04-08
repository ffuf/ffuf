package ffuf

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

//used for random string generation in calibration function
var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

//RandomString returns a random string of length of parameter n
func RandomString(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

//UniqStringSlice returns an unordered slice of unique strings. The duplicates are dropped
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

//FileExists checks if the filepath exists and is not a directory.
//Returns false in case it's not possible to describe the named file.
func FileExists(path string) bool {
	md, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !md.IsDir()
}

//RequestContainsKeyword checks if a keyword is present in any field of a request
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

//Version returns the ffuf version string
func Version() string {
	return fmt.Sprintf("%s%s", VERSION, VERSION_APPENDIX)
}
