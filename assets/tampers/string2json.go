package tamper

import "encoding/json"

type T struct{}

func (t T) Desc() string {
	return "escape payload as a valid JSON string value"
}

func (t T) Exec(payload string) string {
	b, _ := json.Marshal(payload)
	// Remove added prefix and suffix double quotes
	return string(b[1 : len(b)-1])
}

var Tamper T
