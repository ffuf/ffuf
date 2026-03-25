package tamper

import (
	"math/rand"
	"strconv"
)

type T struct{}

func (t T) Desc() string {
	return "replace digits with equivalent random math expressions"
}

func randExpr(n int) string {
	switch rand.Intn(4) {
	case 0:
		// addition
		a := rand.Intn(n + 1)
		b := n - a
		return strconv.Itoa(a) + "+" + strconv.Itoa(b)
	case 1:
		// subtraction
		a := n + rand.Intn(5)
		b := a - n
		return strconv.Itoa(a) + "-" + strconv.Itoa(b)
	case 2:
		// multiplication (simple)
		if n == 0 {
			return "0*1"
		}
		return strconv.Itoa(n) + "*1"
	case 3:
		// division
		if n == 0 {
			return "0/1"
		}
		return strconv.Itoa(n*2) + "/" + strconv.Itoa(2)
	}
	return strconv.Itoa(n)
}

func (t T) Exec(payload string) string {
	out := make([]byte, 0, len(payload)*2)

	for i := 0; i < len(payload); i++ {
		c := payload[i]

		if c >= '0' && c <= '9' {
			n := int(c - '0')
			expr := randExpr(n)
			out = append(out, expr...)
		} else {
			out = append(out, c)
		}
	}

	return string(out)
}

var Tamper T
