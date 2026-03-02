package main

import "strings"

const (
	name = "backslashescape"
)

type tamper struct{}

func (t tamper) Name() string {
	return name
}

func (t tamper) Exec(payload string) string {
	return backslashEscape(payload)
}

func backslashEscape(s string) string {
	var b strings.Builder
	for _, c := range s {
		b.WriteRune('\\')
		b.WriteRune(c)
	}
	return b.String()
}

var Tamper tamper
