package encoder

import (
	"fmt"
)

type EncoderParameters = map[string]string

type encoder func(ep EncoderParameters, data []byte) ([]byte, error)

type encoderRecord struct {
	name           string
	description    string
	implementation encoder
}

type EncoderSpecs = *encoderRecord

func (e EncoderSpecs) Name() string        { return e.name }
func (e EncoderSpecs) Description() string { return e.description }

func Encoders() []EncoderSpecs {
	return []EncoderSpecs{
		&encoderRecord{
			name:           "urlenc",
			description:    "Url encoding. Set 'urlenc_chars=...' to define encoded chars.",
			implementation: urlencode,
		},
		&encoderRecord{
			name:           "b64",
			description:    "Base64. Set 'b64_url=true' for url variant and 'b64_nopad=true' to remove '=' padding.",
			implementation: b64,
		},
	}
}

type encoderInstance struct {
	Specification []string          `json:"encoders,omitempty"`
	Parameters    EncoderParameters `json:"parameters,omitempty"`
	encoder       `json:"-"`
}

type EncoderInstance = *encoderInstance

func (e EncoderInstance) Encode(data []byte) ([]byte, error) {
	return e.encoder(e.Parameters, data)
}

func id(ep EncoderParameters, data []byte) ([]byte, error) {
	return data, nil
}

func Id() EncoderInstance {
	return &encoderInstance{
		Specification: nil,
		Parameters:    nil,
		encoder:       id,
	}
}

func compose(e1, e2 encoder) encoder {
	return func(ep EncoderParameters, data []byte) ([]byte, error) {
		if x, err := e1(ep, data); err != nil {
			return x, err
		} else {
			return e2(ep, x)
		}
	}
}

func Compile(encoders []string, ep EncoderParameters) (EncoderInstance, error) {
	e := id
	for i, enc := range encoders {
		composed := false
		for _, encspec := range Encoders() {
			if encspec.name == enc {
				e = compose(e, encspec.implementation)
				composed = true
				break
			}
		}
		if !composed {
			return nil, fmt.Errorf("Unknown encoder %s at position %d", enc, i)
		}
	}
	return &encoderInstance{
		Specification: encoders,
		Parameters:    ep,
		encoder:       e,
	}, nil
}
