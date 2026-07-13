package ffuf

import (
	"flag"
	"reflect"
	"strings"
)

// Section keys — the help groups, in display order (rendered by help.go).
const (
	SectionHTTP    = "http"
	SectionGeneral = "general"
	SectionCompat  = "compat"
	SectionMatcher = "matcher"
	SectionFilter  = "filter"
	SectionInput   = "input"
	SectionOutput  = "output"
)

// RegisteredFlag is one flag's help metadata.
type RegisteredFlag struct {
	Name    string
	Section string
	Hidden  bool
}

// FlagRegistry is the single source of truth describing every CLI flag, used to
// drive the segmented help output. It is produced by RegisterFlags.
type FlagRegistry struct {
	byName map[string]RegisteredFlag
}

// SectionOf returns the help section a flag belongs to.
func (r *FlagRegistry) SectionOf(name string) (string, bool) {
	f, ok := r.byName[name]
	return f.Section, ok
}

// --- custom flag.Value types, writing straight into the target []string field ---

// multiStringValue appends each occurrence verbatim (e.g. -H "A: b" -H "C: d").
type multiStringValue struct{ p *[]string }

func (m *multiStringValue) String() string { return "" }
func (m *multiStringValue) Set(v string) error {
	*m.p = append(*m.p, v)
	return nil
}

// wordlistValue appends, comma-splitting a single occurrence (e.g. -w a.txt,b.txt).
type wordlistValue struct{ p *[]string }

func (w *wordlistValue) String() string { return "" }
func (w *wordlistValue) Set(v string) error {
	if parts := strings.Split(v, ","); len(parts) > 1 {
		*w.p = append(*w.p, parts...)
	} else {
		*w.p = append(*w.p, v)
	}
	return nil
}

// csvReplaceValue drops any default on the first occurrence, then appends
// comma-split values. Matches the legacy -acs semantics (replace, not append).
type csvReplaceValue struct {
	p   *[]string
	set bool
}

func (c *csvReplaceValue) String() string { return "" }
func (c *csvReplaceValue) Set(v string) error {
	if !c.set {
		*c.p = []string{}
		c.set = true
	}
	*c.p = append(*c.p, strings.Split(v, ",")...)
	return nil
}

// extraFlag is the explicit supplement for flags that can't be expressed as a
// plain tagged field: cross-section aliases, dummy compatibility flags, and
// flags whose usage text contains a backtick (illegal inside a struct tag).
type extraFlag struct {
	Name    string
	Section string
	Hidden  bool
	Bind    func(fs *flag.FlagSet, o *ConfigOptions)
}

var extraFlags = []extraFlag{
	// Backtick-usage flags: their usage strings contain `...`, which cannot live
	// inside a (backtick-delimited) struct tag, so they are declared here.
	{"H", SectionHTTP, false, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.Var(&multiStringValue{&o.HTTP.Headers}, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	}},
	{"b", SectionHTTP, false, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.Var(&multiStringValue{&o.HTTP.Cookies}, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	}},
	{"p", SectionGeneral, false, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.StringVar(&o.General.Delay, "p", o.General.Delay, "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	}},

	// Cross-section aliases (bind the same target as their canonical flag).
	{"cookie", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.Var(&multiStringValue{&o.HTTP.Cookies}, "cookie", "Cookie data (alias of -b)")
	}},
	{"data", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.StringVar(&o.HTTP.Data, "data", o.HTTP.Data, "POST data (alias of -d)")
	}},
	{"data-ascii", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.StringVar(&o.HTTP.Data, "data-ascii", o.HTTP.Data, "POST data (alias of -d)")
	}},
	{"data-binary", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		fs.StringVar(&o.HTTP.Data, "data-binary", o.HTTP.Data, "POST data (alias of -d)")
	}},

	// Dummy compatibility flags (accepted and ignored).
	{"compressed", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		_ = fs.Bool("compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	}},
	{"i", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		_ = fs.Bool("i", true, "Dummy flag for copy as curl functionality (ignored)")
	}},
	{"k", SectionCompat, true, func(fs *flag.FlagSet, o *ConfigOptions) {
		_ = fs.Bool("k", false, "Dummy flag for backwards compatibility")
	}},
}

// RegisterFlags is the single source of truth for CLI flag registration. It walks
// the `ffuf:`-tagged fields of ConfigOptions via reflection and registers each on
// fs — using the field's CURRENT value as the default, which preserves the
// config-file < command-line precedence — then applies the explicit supplement.
// It returns a FlagRegistry describing every flag for the segmented help.
func RegisterFlags(fs *flag.FlagSet, o *ConfigOptions) *FlagRegistry {
	reg := &FlagRegistry{byName: make(map[string]RegisteredFlag)}
	record := func(name, section string, hidden bool) {
		reg.byName[name] = RegisteredFlag{Name: name, Section: section, Hidden: hidden}
	}

	root := reflect.ValueOf(o).Elem()
	for i := 0; i < root.NumField(); i++ {
		group := root.Field(i) // an *Options sub-struct (addressable: o is a pointer)
		gt := group.Type()
		for j := 0; j < gt.NumField(); j++ {
			sf := gt.Field(j)
			name := sf.Tag.Get("ffuf")
			if name == "" {
				continue // not a flag
			}
			bindField(fs, name, sf.Tag.Get("usage"), sf.Tag.Get("kind"), group.Field(j).Addr().Interface())
			record(name, sf.Tag.Get("section"), false)
		}
	}

	for _, ef := range extraFlags {
		ef.Bind(fs, o)
		record(ef.Name, ef.Section, ef.Hidden)
	}
	return reg
}

// bindField registers one reflected field pointer as a flag of the matching type.
// It panics on an unsupported type or a slice field without a valid kind — those
// are programming errors caught by the registration test, not runtime conditions.
func bindField(fs *flag.FlagSet, name, usage, kind string, ptr interface{}) {
	switch p := ptr.(type) {
	case *bool:
		fs.BoolVar(p, name, *p, usage)
	case *int:
		fs.IntVar(p, name, *p, usage)
	case *string:
		fs.StringVar(p, name, *p, usage)
	case *[]string:
		switch kind {
		case "multistring":
			fs.Var(&multiStringValue{p}, name, usage)
		case "wordlist":
			fs.Var(&wordlistValue{p}, name, usage)
		case "csvreplace":
			fs.Var(&csvReplaceValue{p: p}, name, usage)
		default:
			panic("ffuf: slice flag -" + name + " needs kind:multistring|wordlist|csvreplace (got " + kind + ")")
		}
	default:
		panic("ffuf: unsupported flag field type for -" + name)
	}
}
