package filter

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type HashFilter struct {
	Value []string
}

func NewHashFilter(value string) (ffuf.FilterProvider, error) {
	var hashes []string
	for _, h := range strings.Split(value, ",") {
		hashes = append(hashes, strings.TrimSpace(h))
	}
	return &HashFilter{Value: hashes}, nil
}

func (f *HashFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Value string `json:"value"`
	}{
		Value: strings.Join(f.Value, ","),
	})
}

func CalculateHash(resp *ffuf.Response) string {
	hasher := md5.New()
	hasher.Write(resp.Data)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (f *HashFilter) Filter(response *ffuf.Response) (bool, error) {
	currentHash := CalculateHash(response)
	for _, targetHash := range f.Value {
		if currentHash == targetHash {
			return true, nil
		}
	}
	return false, nil
}

func (f *HashFilter) Repr() string {
	return strings.Join(f.Value, ",")
}

func (f *HashFilter) ReprVerbose() string {
	return fmt.Sprintf("Response hash: %s", f.Repr())
}
