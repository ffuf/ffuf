package payloadtamper

type adapter struct {
	name string
	desc string
	exec func(string) string
}

func (a *adapter) Name() string         { return a.name }
func (a *adapter) Desc() string         { return a.desc }
func (a *adapter) Exec(s string) string { return a.exec(s) }
