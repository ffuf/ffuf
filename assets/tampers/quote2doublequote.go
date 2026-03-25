package tamper

type T struct{}

func (t T) Desc() string {
	return "replace single quotes with double quotes"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		if payload[i] == '\'' {
			buf[i] = '"'
		} else {
			buf[i] = payload[i]
		}
	}
	return string(buf)
}

var Tamper T
