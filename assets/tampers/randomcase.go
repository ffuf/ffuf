package tamper

import "math/rand"

type T struct{}

func (t T) Desc() string {
	return "mix upper and lower case with the characters [a-zA-Z] (\"RanDoMCAsE\")"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		c := payload[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			if rand.Intn(2) == 0 {
				c |= 0x20 // lowercase
			} else {
				c &^= 0x20 // uppercase
			}
		}
		buf[i] = c
	}
	return string(buf)
}

var Tamper T
