package utils

import (
	"math/rand"
	"os"
	"strings"
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

// TokenizeArgs splits a commandline like string into an os.Args compatible
// string slice. It splits it on whitespace except where the tokens are enclosed
// in single or double quotes. These are considered one token without recursing
// on them, e.g. if the token starts with a single quote, any double quote inside
// it is considered part of the token.
func TokenizeArgs1(args string) []string {

	tokens := make([]string, 0, 20)
	sb := new(strings.Builder)

	getNextToken := func(seek ...rune) (token string) {
		var (
			pos int
			r   rune
		)

	outer:
		for pos, r = range args {
			for _, stop := range seek {
				if r == stop {
					break outer
				}
			}
			sb.WriteRune(r)
		}

		token = sb.String()
		sb.Reset()
		args = args[pos+1:]

		return token
	}

	for args != "" {
		var b byte = args[0]
		switch b {
		case 9, 32: // tab or space
			args = args[1:]
		case 34, 39: // double quote
			args = args[1:]
			tokens = append(tokens, getNextToken(rune(b)))
		default: // normal char, seek tab or space or EOF
			tokens = append(tokens, getNextToken(rune(9), rune(32)))
		}
	}

	return tokens
}

func TokenizeArgs2(args string) (tokens []string, err error) {
	tokens = make([]string, 0, 20)

	var (
		seeking = -1 // 0 means seeking nothing and splitting on white space
		sb      = new(strings.Builder)
		escape  = false
	)

	for _, r := range args {
		switch r {
		case 9, 32: // tab, space
			if seeking == -1 && !escape { // neither seeking quotes nor escaping
				if sb.Len() != 0 { // end of token found, else whitespace is skipped
					tokens = append(tokens, sb.String())
					sb.Reset()
				}
			} else { // either seeking a quote or escaping: write whitespace
				sb.WriteRune(r)
				escape = false
			}
		case 34: // double quote
			if seeking == 34 || !escape {
				tokens = append(tokens, sb.String())
				sb.Reset()
				seeking = -1
			} else {
				sb.WriteRune(r)
			}
		case 39: // single quote
			if seeking == 39 && !escape {
				tokens = append(tokens, sb.String())
				sb.Reset()
				seeking = -1
			} else {
				sb.WriteRune(r)
				escape = false
			}

		case 92: // backslash: escape char
			escape = true

		default: // writing to the string builder
			sb.WriteRune(r)
		}
	}

	return tokens, nil
}
