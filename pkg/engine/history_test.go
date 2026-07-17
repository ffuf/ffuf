package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestConfigOptions_SerializationRoundTrip locks the serialize/deserialize path
// that FFUFHASH history (and similar features) rely on. A Config built from
// options retains those options (conf.Options); serializing them to JSON and
// reading them back must rebuild an equivalent request. This is the automatic
// replacement for the former hand-maintained ToOptions reverse mapper — adding a
// ConfigOptions field is picked up here for free, with no marshaller to update.
func TestConfigOptions_SerializationRoundTrip(t *testing.T) {
	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = "https://example.org/FUZZ"
	opts.HTTP.Method = "POST"
	opts.HTTP.Data = "name=FUZZ"
	opts.HTTP.Headers = []string{"X-Test: FUZZ"}
	opts.Input.Wordlists = []string{"/tmp/wl.txt"}
	opts.General.Threads = 7
	opts.General.Delay = "0.1-0.8"

	conf, err := ffuf.ConfigFromOptions(opts, context.Background(), func() {})
	if err != nil {
		t.Fatalf("ConfigFromOptions: %v", err)
	}
	if conf.Options == nil {
		t.Fatal("conf.Options was not retained by ConfigFromOptions")
	}

	// Serialize exactly as WriteHistoryEntry does, then read back.
	blob, err := json.Marshal(conf.Options)
	if err != nil {
		t.Fatalf("marshal source options: %v", err)
	}
	var restored ffuf.ConfigOptions
	if err := json.Unmarshal(blob, &restored); err != nil {
		t.Fatalf("unmarshal options: %v", err)
	}

	// Rebuilding from the restored options must reproduce the request.
	conf2, err := ffuf.ConfigFromOptions(&restored, context.Background(), func() {})
	if err != nil {
		t.Fatalf("ConfigFromOptions (rebuild): %v", err)
	}

	if conf2.Url != conf.Url {
		t.Errorf("url changed across round-trip: %q -> %q", conf.Url, conf2.Url)
	}
	if conf2.Method != conf.Method {
		t.Errorf("method changed across round-trip: %q -> %q", conf.Method, conf2.Method)
	}
	if conf2.Data != conf.Data {
		t.Errorf("data changed across round-trip: %q -> %q", conf.Data, conf2.Data)
	}
	if conf2.Threads != conf.Threads {
		t.Errorf("threads changed across round-trip: %d -> %d", conf.Threads, conf2.Threads)
	}
	if conf2.Delay.Min != conf.Delay.Min || conf2.Delay.Max != conf.Delay.Max {
		t.Errorf("delay changed across round-trip: %+v -> %+v", conf.Delay, conf2.Delay)
	}
	if conf2.Headers["X-Test"] != conf.Headers["X-Test"] {
		t.Errorf("header changed across round-trip: %q -> %q", conf.Headers["X-Test"], conf2.Headers["X-Test"])
	}
}

// --- test doubles for the runtime-derived serialization state ---

type fakeFilter struct{ repr string }

func (f fakeFilter) Filter(*ffuf.Response) (bool, error) { return false, nil }
func (f fakeFilter) Repr() string                        { return f.repr }
func (f fakeFilter) ReprVerbose() string                 { return f.repr }

type fakeMatcherManager struct {
	filters  map[string]ffuf.FilterProvider
	matchers map[string]ffuf.FilterProvider
}

func (m *fakeMatcherManager) SetCalibrated(bool)                                     {}
func (m *fakeMatcherManager) SetCalibratedForHost(string, bool)                      {}
func (m *fakeMatcherManager) AddFilter(string, string, bool) error                   { return nil }
func (m *fakeMatcherManager) AddPerDomainFilter(string, string, string) error        { return nil }
func (m *fakeMatcherManager) RemoveFilter(string)                                    {}
func (m *fakeMatcherManager) AddMatcher(string, string) error                        { return nil }
func (m *fakeMatcherManager) GetFilters() map[string]ffuf.FilterProvider             { return m.filters }
func (m *fakeMatcherManager) GetMatchers() map[string]ffuf.FilterProvider            { return m.matchers }
func (m *fakeMatcherManager) FiltersForDomain(string) map[string]ffuf.FilterProvider { return nil }
func (m *fakeMatcherManager) CalibratedForDomain(string) bool                        { return false }
func (m *fakeMatcherManager) Calibrated() bool                                       { return false }
func (m *fakeMatcherManager) Matches(*ffuf.Response, bool, string, string) bool      { return false }

// TestHistoryOptions_ReflectsRecursedURL locks the recursion fix: WriteHistoryEntry
// serializes the LIVE Config.Url (rewritten per queued job by prepareQueueJob), not
// the base URL frozen in the retained parse-time options. Without the fix, -search on
// a recursive hit reconstructs the wrong (base) path.
func TestHistoryOptions_ReflectsRecursedURL(t *testing.T) {
	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = "https://example.org/FUZZ" // parse-time base
	conf := &ffuf.Config{Options: opts, Url: "https://example.org/admin/FUZZ"}

	got := historyOptions(conf)
	if got.HTTP.URL != "https://example.org/admin/FUZZ" {
		t.Errorf("history URL = %q, want the recursed live URL %q", got.HTTP.URL, "https://example.org/admin/FUZZ")
	}
}

// TestHistoryOptions_ReflectsRuntimeFilters locks the autocalibration fix: filters
// installed at runtime (e.g. by -ac) appear in the serialized history, rebuilt from
// MatcherManager. The raw parse-time options never held them.
func TestHistoryOptions_ReflectsRuntimeFilters(t *testing.T) {
	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = "https://example.org/FUZZ"
	conf := &ffuf.Config{
		Options: opts,
		Url:     "https://example.org/FUZZ",
		MatcherManager: &fakeMatcherManager{
			filters: map[string]ffuf.FilterProvider{"size": fakeFilter{"4242"}},
		},
	}

	got := historyOptions(conf)
	if got.Filter.Size != "4242" {
		t.Errorf("history Filter.Size = %q, want the runtime-installed %q", got.Filter.Size, "4242")
	}
}
