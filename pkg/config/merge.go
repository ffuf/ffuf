// functionality concerned with merging ConfigOptions

package config

import (
	"fmt"
	"reflect"
)

/////////////
// EXPORTS //
/////////////

// Merge takes an arbitrary amount of ConfigOptions references and
// merged them into one, where the next argument's options overwrite the prior
// ones. This creates an explicit hierarchoy of configurations (in ascending
// order). Exceptions are default options as definded by NewConfigOptions(),
// which are not merged, even if they come later and slice type options, which
// elements are deduplicated and appended.
// Merge does not mutate it's arguments.
func Merge(opts ...*ConfigOptions) (merged *ConfigOptions) {
	merged = NewConfigOptions()

	for _, opt := range opts {
		if opt != nil {
			// opt might be nil, if a config source is not available and should
			// be skipped
			merged.mergeIfNotDefault(opt)
		}
	}

	return merged
}

///////////////
// AUXILIARY //
///////////////

// mergeIfNotDefault() merges new into prior (both of type ConfigOptions) if new
// deviates from the default value as defined by NewConfigOptions(). It does so
// via reflection to avoid having another place to update when the struct of
// ConfigOptions changes.
func (prior *ConfigOptions) mergeIfNotDefault(new *ConfigOptions) {

	var (
		prior_v   = reflect.ValueOf(prior)
		new_v     = reflect.ValueOf(new)
		default_v = reflect.ValueOf(NewConfigOptions())
	)

	merge(prior_v.Elem(), new_v.Elem(), default_v.Elem())
}

// Merge recurses nested structs until it encounters a value of the types the
// flag library uses to parse command line arguments (which are thus reflected
// in the ConfigOptions struct) and replaces the value of prior with the
// value of new, if the new value deviates from the defaults defined in def.
// Merge deduplicates and concatenates slices instead of overwriting them.
// Merge merges maps.
func merge(prior, new, def reflect.Value) {

	switch prior.Kind() {
	case reflect.Invalid:
		panic(
			fmt.Sprintf(
				"merge: error merging config: encountered invalid type %q with value %q",
				prior.Type().String(), prior.String()))

	case reflect.Bool, reflect.Int, reflect.Int64, reflect.Float64, reflect.String, reflect.Uint, reflect.Uint64:
		// for data types in flag library: merge
		if prior.CanSet() && new.Interface() != def.Interface() {
			prior.Set(new)
		}

	case reflect.Struct:
		// Recurse for every field in the sub-struct.
		for i := 0; i < prior.NumField(); i++ {
			merge(prior.Field(i), new.Field(i), def.Field(i))
		}

	case reflect.Ptr:
		// Dereference and retry
		if prior.IsNil() || new.IsNil() || def.IsNil() {
			panic(fmt.Sprintf("merge: encountered nil pointer in %s", prior.Type().String()))
		}

		merge(prior.Elem(), new.Elem(), def.Elem())

	case reflect.Slice:
		if prior.IsNil() || new.IsNil() || def.IsNil() {
			panic(fmt.Sprintf("merge: encountered nil slice in %s", prior.Type().String()))
		}

		// merge unique values
	next:
		for i := 0; i < new.Len(); i++ {
			elem := new.Index(i)

			// check if elem is already present
			for j := 0; j < prior.Len(); j++ {
				if reflect.DeepEqual(elem.Interface(), prior.Index(j).Interface()) {
					// if elem is found inside prior, do not append it to prior
					// and continue with next element of new
					continue next
				}
			}
			// elem is not found inside prior: append to prior
			prior.Set(reflect.Append(prior, elem))
		}

	case reflect.Map:
		if prior.IsNil() || new.IsNil() {
			panic(fmt.Sprintf("merge: encountered nil map in %s", prior.Type().String()))
		}

		new_it := new.MapRange()

		for new_it.Next() {
			k := new_it.Key()
			v := new_it.Value()
			prior.SetMapIndex(k, v)
		}

	default:
		// handling of reflect.Array, reflect.Interface and other unsupported types
		panic(
			fmt.Sprintf(
				"merge: error merging config: encountered unsupported type  %s",
				prior.Type().String()))
	}
}
