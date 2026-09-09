package scraper

import (
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func headerString(headers map[string][]string) string {
	// Built with a Builder rather than repeated concatenation: the headers come
	// from the scanned target, and += in this nested loop copies the accumulated
	// string on every header value.
	var sb strings.Builder
	for k, vslice := range headers {
		for _, v := range vslice {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func isActive(name string, activegroups []string) bool {
	return ffuf.StrInSlice(strings.ToLower(strings.TrimSpace(name)), activegroups)
}

func parseActiveGroups(activestr string) []string {
	retslice := make([]string, 0)
	for _, v := range strings.Split(activestr, ",") {
		retslice = append(retslice, strings.ToLower(strings.TrimSpace(v)))
	}
	return retslice
}
