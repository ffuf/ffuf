package scraper

import "fmt"

func headerString(headers map[string][]string) string {
	val := ""
	for k, vslice := range headers {
		for _, v := range vslice {
			val += fmt.Sprintf("{}: {}\n", k, v)
		}
	}
	return val
}
