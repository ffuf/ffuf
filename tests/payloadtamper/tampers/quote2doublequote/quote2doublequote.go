package main

import "strings"

const (
	name = "quote2doublequote"
)

type tamper struct{}

func (t tamper) Name() string {
	return name
}

func (t tamper) Exec(payload string) string {
	return strings.ReplaceAll(payload, "'", "\"")
}

var Tamper tamper
