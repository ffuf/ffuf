package tamper

type T struct{}

func (t T) Desc() string {
	return "replace the keyword \"alert\" with \"prompt\" (Ex: alert(...) => prompt(...))"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload))
	for i := 0; i < len(payload); i++ {
		if i+5 <= len(payload) && payload[i:i+5] == "alert" {
			buf = append(buf, 'p', 'r', 'o', 'm', 'p', 't')
			i += 4
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
