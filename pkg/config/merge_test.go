package config

import (
	"fmt"
	"reflect"
	"testing"
)

/////////////////////////////////////////
// Test merge with with a dummy struct //
/////////////////////////////////////////

// dummy is a struct type holding all the data types supported by the flag library
// and also nested structs and pointers to structs.
// It's purpose is to test the config.merge() function.
type dummy struct {
	Flag      bool
	Int       int
	Int64     int64
	Float64   float64
	String    string
	Uint      uint
	Uint64    uint64
	SliceInt  []int
	SliceStr  []string
	Nested    nested
	NestedPtr *nested
	Map       map[string]string
}

type nested struct {
	String    string
	Float64   float64
	invisible uint
}

func (left *dummy) IsEqual(right *dummy) (ok bool) {
	if left.Flag != right.Flag {
		return false
	}
	if left.Int != right.Int {
		return false
	}
	if left.Int64 != right.Int64 {
		return false
	}
	if left.Float64 != right.Float64 {
		return false
	}
	if left.String != right.String {
		return false
	}
	if left.Uint != right.Uint {
		return false
	}
	if left.Uint64 != right.Uint64 {
		return false
	}

	if len(left.SliceInt) != len(right.SliceInt) {
		return false
	}
	for i, elem := range left.SliceInt {
		if elem != right.SliceInt[i] {
			return false
		}
	}

	if len(left.SliceStr) != len(right.SliceStr) {
		return false
	}
	for i, elem := range left.SliceStr {
		if elem != right.SliceStr[i] {
			return false
		}
	}

	if left.Nested.String != right.Nested.String {
		return false
	}
	if left.Nested.Float64 != right.Nested.Float64 {
		return false
	}
	if left.NestedPtr.String != right.NestedPtr.String {
		return false
	}
	if left.NestedPtr.Float64 != right.NestedPtr.Float64 {
		return false
	}
	// check map equality
	for k, v := range left.Map {
		if v != right.Map[k] {
			return false
		}
	}

	return true
}

// TestMergeGeneric tests the merge function on a dummy struct
// merge() should handle the invisible field well.
func TestMerge(t *testing.T) {

	var def = &dummy{false, -2, 123456, 3.14, "", 0, 0, []int{0}, []string{}, nested{"_", 1.0, 0}, &nested{"", 0.0, 0}, map[string]string{"def": "yes"}}

	tests := []struct {
		Prior *dummy
		New   *dummy
		Want  *dummy
	}{
		{ // default config to be merged: Prior shoud stay the same, with the slice and map types merged
			&dummy{true, -2, 1, 3.14, "halloo", 0, 0, []int{1, 2}, []string{"hi", "hallo"}, nested{"~", 1.2, 0}, &nested{"-", 0.1, 0}, map[string]string{"a": "b"}},
			&dummy{false, -2, 123456, 3.14, "", 0, 0, []int{0}, []string{}, nested{"_", 1.0, 0}, &nested{"", 0.0, 0}, map[string]string{"def": "yes"}},
			&dummy{true, -2, 1, 3.14, "halloo", 0, 0, []int{1, 2, 0}, []string{"hi", "hallo"}, nested{"~", 1.2, 0}, &nested{"-", 0.1, 0}, map[string]string{"def": "yes", "a": "b"}},
		},
		{ // an arbitrary mix
			&dummy{false, 14, 16, 0.14, "", 0, 0, []int{1, 8, 7}, []string{"hi"}, nested{"hello", 1.2, 0}, &nested{"", 0.0, 0}, map[string]string{"a": "b", "c": "d"}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{1, 2, 8}, []string{"yo", "hey"}, nested{"_", 1.0, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{"a": "b", "c": "e"}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{1, 8, 7, 2}, []string{"hi", "yo", "hey"}, nested{"hello", 1.2, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{"a": "b", "c": "e"}},
		},
		{ // with focus on slices and maps
			&dummy{false, 14, 16, 0.14, "", 0, 0, []int{1, 1, 2, 3}, []string{"hi"}, nested{"hello", 1.2, 0}, &nested{"", 0.0, 0}, map[string]string{"a": "b", "c": "d"}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{2, 3, 4, 2}, []string{"hi", "hi", "yo"}, nested{"_", 1.0, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{"e": "f"}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{1, 1, 2, 3, 4}, []string{"hi", "yo"}, nested{"hello", 1.2, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{"a": "b", "c": "d", "e": "f"}},
		},
		{ // empty slices and maps
			&dummy{false, 14, 16, 0.14, "", 0, 0, []int{1, 1, 2, 3}, []string{}, nested{"hello", 1.2, 0}, &nested{"", 0.0, 0}, map[string]string{}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{2, 3, 4, 2}, []string{}, nested{"_", 1.0, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{}},
			&dummy{true, 14, 3456, 34.0, "", 0, 0, []int{1, 1, 2, 3, 4}, []string{}, nested{"hello", 1.2, 0}, &nested{"asdf", 123124325.7, 0}, map[string]string{}},
		},
	}

	for _, test := range tests {
		merge(
			reflect.ValueOf(test.Prior),
			reflect.ValueOf(test.New),
			reflect.ValueOf(def),
		)

		if !test.Prior.IsEqual(test.Want) {
			t.Errorf(fmt.Sprintf(
				"prior is not equal to want after merge.\n\tprior: %v, nested: %v, *nested: %v\n\t  new: %v, nested: %v, *nested: %v\n\t want: %v, nested: %v, *nested: %v",
				test.Prior, test.Prior.Nested, test.Prior.NestedPtr,
				test.New, test.New.Nested, test.New.NestedPtr,
				test.Want, test.Want.Nested, test.Want.NestedPtr,
			))
		}
	}
}

///////////////////////////////////////
// Test merge with unsupported  type //
///////////////////////////////////////

type invalid struct {
	Valid   string
	Invalid interface{}
}

// TestMergeInvalidTypes tests if merge panics on invalid types.
func TestMergeInvalidTypes(t *testing.T) {

	type empty interface{}
	e := new(empty)

	tests := []struct {
		Prior *invalid
		New   *invalid
	}{
		{
			&invalid{"hallo", e},
			&invalid{"hi", e},
		},
	}

	for _, test := range tests {
		defer func() {
			if r := recover(); r == nil {
				t.Error("merge did not panic on invalid type")
			}
		}()

		merge(
			reflect.ValueOf(test.Prior),
			reflect.ValueOf(test.New),
			reflect.ValueOf(new(invalid)),
		)
	}
}

///////////////////////////////////////////
// Test merge with ConfigOptions structs //
///////////////////////////////////////////

// TestMergeConfigOptionsSample tests the merging of ConfigOptions on a few
// sample fields. This test can be quickly adopted to test the merging of a
// specific field, like a new one.
func TestMergeConfigOptionsSample(t *testing.T) {

	prior := NewConfigOptions()
	new := NewConfigOptions()
	want := NewConfigOptions()

	// do not accept default value
	prior.Filter.Mode = "test"
	// new.Filter.Mode stays default
	want.Filter.Mode = "test"

	// merge AutoCalibrationStrings
	prior.General.AutoCalibrationStrings = []string{"a", "b"}
	new.General.AutoCalibrationStrings = []string{"b", "c"}
	want.General.AutoCalibrationStrings = []string{"a", "b", "c"}

	// overwrite flag
	prior.HTTP.FollowRedirects = false
	new.HTTP.FollowRedirects = true
	want.HTTP.FollowRedirects = true

	// no change
	prior.Input.InputNum = 3
	new.Input.InputNum = 3
	want.Input.InputNum = 3

	prior.mergeIfNotDefault(new)

	// lazy shortcut
	vp := reflect.ValueOf(prior)
	vw := reflect.ValueOf(want)

	if !reflect.DeepEqual(vp.Interface(), vw.Interface()) {
		t.Error("merging failed: prior and want differ.")
	}
}
