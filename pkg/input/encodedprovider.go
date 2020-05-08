package input

import (
	"log"

	"github.com/ffuf/ffuf/pkg/encoder"
	"github.com/ffuf/ffuf/pkg/ffuf"
)

type encodedInputProvider struct {
	iip ffuf.InternalInputProvider
	ei  encoder.EncoderInstance
}

func (eip *encodedInputProvider) Keyword() string    { return eip.iip.Keyword() }
func (eip *encodedInputProvider) Next() bool         { return eip.iip.Next() }
func (eip *encodedInputProvider) Position() int      { return eip.iip.Position() }
func (eip *encodedInputProvider) ResetPosition()     { eip.iip.ResetPosition() }
func (eip *encodedInputProvider) IncrementPosition() { eip.iip.IncrementPosition() }
func (eip *encodedInputProvider) Total() int         { return eip.iip.Total() }

func (eip *encodedInputProvider) Value() []byte {
	data, err := eip.ei.Encode(eip.iip.Value())
	log.Printf("wht is this \n")
	if err == nil {
		return data
	}

	log.Printf("Encoder error: %s: %s\n", eip.iip.Keyword(), err)

	// not possible to return an error from this interface
	return []byte{}
}
