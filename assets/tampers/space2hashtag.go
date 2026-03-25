package tamper

type T struct{}

func (t T) Desc() string {
	return "replace space with hashtag comment and newline (\"#\\n\")"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload)*2)
	for i := 0; i < len(payload); i++ {
		if payload[i] == ' ' {
			buf = append(buf, '#', '\n')
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
