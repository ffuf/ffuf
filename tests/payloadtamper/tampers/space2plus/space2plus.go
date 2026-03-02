package main

import "strings"

const (
	name = "space2plus"
)

type tamper struct{}

func (t tamper) Name() string {
	return "space2plus"
}

func (t tamper) Exec(payload string) string {
	return strings.ReplaceAll(payload, " ", "+")
}

var Tamper tamper
