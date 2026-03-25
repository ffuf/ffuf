package tamper

type T struct{}

func (t T) Desc() string {
	return "replace percentage \"%\" with \"%25\""
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload)*2)
	for i := 0; i < len(payload); i++ {
		if payload[i] == '%' {
			buf = append(buf, '%', '2', '5')
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
