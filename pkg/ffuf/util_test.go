package ffuf

import (
	"math/rand"
	"testing"
)

func TestRandomString(t *testing.T) {
	length := 1 + rand.Intn(65535)
	str := RandomString(length)

	if len(str) != length {
		t.Errorf("Length of generated string was %d, was expecting %d", len(str), length)
	}
}

func TestUniqStringSlice(t *testing.T) {
	slice := []string{"foo", "foo", "bar", "baz", "baz", "foo", "baz", "baz", "foo"}
	expectedLength := 3

	uniqSlice := UniqStringSlice(slice)

	if len(uniqSlice) != expectedLength {
		t.Errorf("Length of slice was %d, was expecting %d", len(uniqSlice), expectedLength)
	}
}
