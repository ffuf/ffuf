package tamper

type T struct{}

func (t T) Desc() string {
	return "replace CRLF (\\r\\n) with newline (\"\\n\")"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload))
	for i := 0; i < len(payload); i++ {
		if payload[i] == '\r' && i+1 < len(payload) && payload[i+1] == '\n' {
			buf = append(buf, '\n')
			i++
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
