package tamper

type T struct{}

func (t T) Desc() string {
	return "append null byte urlencoded (\"%00\") to payload"
}

func (t T) Exec(payload string) string {
	return payload + "%00"
}

var Tamper T
