package tamper

type T struct{}

func (t T) Desc() string {
	return "replace the keyword \"http://\" with \"https://\" (Ex: http://example.com/ => https://example.com)"
}

func (t T) Exec(payload string) string {
	buf := make([]byte, 0, len(payload))
	for i := 0; i < len(payload); i++ {
		if i+7 <= len(payload) && payload[i:i+7] == "http://" {
			buf = append(buf, 'h', 't', 't', 'p', 's', ':', '/', '/')
			i += 6
		} else {
			buf = append(buf, payload[i])
		}
	}
	return string(buf)
}

var Tamper T
