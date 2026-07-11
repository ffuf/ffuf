package tamper

type T struct{}

func (t T) Desc() string {
	return "make all characters lowercase [a-zA-Z] (\"lowercase\")"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		c := payload[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			c |= 0x20 // lowercase
		}
		buf[i] = c
	}
	return string(buf)
}

var Tamper T
