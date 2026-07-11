package tamper

type T struct{}

func (t T) Desc() string {
	return "replace newline (\\n) with CRLF (\"\\r\\n\")"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload)*2)
	for i := 0; i < len(payload); i++ {
		if payload[i] == '\n' {
			buf = append(buf, '\r', '\n')
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
