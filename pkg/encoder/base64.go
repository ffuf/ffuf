package encoder

import (
	"encoding/base64"
)

func b64(ep EncoderParameters, data []byte) ([]byte, error) {
	encs := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
	}
	if val, ok := ep["b64_url"]; ok && val == "true" {
		encs = []*base64.Encoding{
			base64.URLEncoding,
			base64.RawURLEncoding,
		}
	}

	enc := encs[0]
	if val, ok := ep["b64_nopad"]; ok && val == "true" {
		enc = encs[1]
	}

	encoded := make([]byte, enc.EncodedLen(len(data)))
	enc.Encode(encoded, data)
	return encoded, nil
}
