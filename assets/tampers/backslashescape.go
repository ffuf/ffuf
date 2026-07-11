package tamper

type T struct{}

func (t T) Desc() string {
	return "add a backslash before characters"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload)*2)
	for i := 0; i < len(payload); i++ {
		buf = append(buf, '\\', payload[i])
	}
	return string(buf)
}

var Tamper T
