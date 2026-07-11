package tamper

type T struct{}

func (t T) Desc() string {
	return "add prefix \"yeswehack_\""
}

func (t T) Exec(payload string) string {
	return "yeswehack_" + payload
}

var Tamper T
