package tamper

type T struct{}

func (t T) Desc() string {
	return "replace parentheses with backticks (Ex: alert(1) => alert`1`)"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		if payload[i] == '(' || payload[i] == ')' {
			buf[i] = '`'
		} else {
			buf[i] = payload[i]
		}
	}
	return string(buf)
}

var Tamper T
