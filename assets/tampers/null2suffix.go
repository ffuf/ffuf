package tamper

type T struct{}

func (t T) Desc() string {
	return "append null byte (\"\\0\") to payload"
}

func (t T) Exec(payload string) string {
	return payload + "\x00"
}

var Tamper T
