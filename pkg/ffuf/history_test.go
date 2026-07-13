package ffuf

import (
	"context"
	"encoding/json"
	"testing"
)

// TestConfigOptions_SerializationRoundTrip locks the serialize/deserialize path
// that FFUFHASH history (and similar features) rely on. A Config built from
// options retains those options (conf.Options); serializing them to JSON and
// reading them back must rebuild an equivalent request. This is the automatic
// replacement for the former hand-maintained ToOptions reverse mapper — adding a
// ConfigOptions field is picked up here for free, with no marshaller to update.
func TestConfigOptions_SerializationRoundTrip(t *testing.T) {
	opts := NewConfigOptions()
	opts.HTTP.URL = "https://example.org/FUZZ"
	opts.HTTP.Method = "POST"
	opts.HTTP.Data = "name=FUZZ"
	opts.HTTP.Headers = []string{"X-Test: FUZZ"}
	opts.Input.Wordlists = []string{"/tmp/wl.txt"}
	opts.General.Threads = 7
	opts.General.Delay = "0.1-0.8"

	conf, err := ConfigFromOptions(opts, context.Background(), func() {})
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
	var restored ConfigOptions
	if err := json.Unmarshal(blob, &restored); err != nil {
		t.Fatalf("unmarshal options: %v", err)
	}

	// Rebuilding from the restored options must reproduce the request.
	conf2, err := ConfigFromOptions(&restored, context.Background(), func() {})
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
