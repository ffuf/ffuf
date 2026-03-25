package tamper

type T struct{}

func (t T) Desc() string {
	return "add %BF before quotes (\", ')"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload)*2)
	for i := 0; i < len(payload); i++ {
		c := payload[i]
		if c == '"' || c == '\'' {
			buf = append(buf, '%', 'B', 'F')
		}
		buf = append(buf, c)
	}
	return string(buf)
}

var Tamper T
