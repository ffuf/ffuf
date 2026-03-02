package tests

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/payloadtamper"
)

func Test_payloadTamper(T *testing.T) {
	// Make sure to compile the tampers before running, Ex:
	// Command: go build -buildmode=plugin -o ./tampers/backslashescape.so ./tampers/backslashescape/backslashescape.go
	var (
		payloadOriginal = "' or 1=1 -- "
		directory       = "./tampers"
		tampers         = []string{"backslashescape"} //[]string{"quote2doublequote", "space2plus", "backslashescape"}
	)

	payloadTamper, err := payloadtamper.New(payloadtamper.Config{
		Directory: directory,
		Tampers:   tampers,
	})
	if err != nil {
		log.Fatal(err)
	}
	err = payloadTamper.LoadTampers()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(strings.Join(payloadTamper.GetTampers(), "\n"))

	payload := payloadTamper.Execute(payloadOriginal)

	fmt.Println(payload)
}
