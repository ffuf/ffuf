package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// gzipBomb returns a gzip stream that decompresses to `decompressed` bytes of
// zeros. The compressed form is tiny (zeros compress ~1000x), so the test moves
// almost no data over the wire while forcing a large decompression.
func gzipBomb(t *testing.T, decompressed int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	chunk := make([]byte, 1<<20) // 1 MiB of zeros, reused
	for written := 0; written < decompressed; {
		n := len(chunk)
		if r := decompressed - written; r < n {
			n = r
		}
		if _, err := zw.Write(chunk[:n]); err != nil {
			t.Fatalf("gzip write: %v", err)
		}
		written += n
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func runnerFor(t *testing.T, url string) (ffuf.RunnerProvider, ffuf.Request) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Method = "GET"
	runner := NewSimpleRunner(&conf, false)
	req := ffuf.NewRequest(&conf)
	req.Method = "GET"
	req.Url = url
	return runner, req
}

// A malicious server returning a decompression bomb must not be read into memory
// unbounded. The response is capped at MAX_DOWNLOAD_SIZE and marked Cancelled
// rather than OOM-killing the process. Regression test for the gzip-bomb DoS
// (CWE-409): before the fix, io.ReadAll on the decompressed stream had no bound.
func TestExecute_GzipBombIsCapped(t *testing.T) {
	const decompressed = 64 << 20 // 64 MiB, well over the 5 MiB cap
	bomb := gzipBomb(t, decompressed)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(bomb)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bomb)
	}))
	defer srv.Close()

	// Path 1: default client. The transport adds Accept-Encoding: gzip itself,
	// transparently decodes, and strips Content-Encoding/Content-Length, so the
	// header-based guard never fires. The LimitReader is what saves us.
	t.Run("transparent transport decode", func(t *testing.T) {
		runner, req := runnerFor(t, srv.URL+"/")
		resp, err := runner.Execute(&req)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		if !resp.Cancelled {
			t.Fatalf("expected cancelled response, got %d bytes of data", len(resp.Data))
		}
		if len(resp.Data) > MAX_DOWNLOAD_SIZE {
			t.Fatalf("read %d bytes, must not exceed MAX_DOWNLOAD_SIZE (%d)", len(resp.Data), MAX_DOWNLOAD_SIZE)
		}
	})

	// Path 2: client sends its own Accept-Encoding, so the transport does NOT
	// auto-decode. Content-Encoding: gzip survives and Content-Length reflects the
	// small compressed size, passing the guard, so the manual decoder + LimitReader
	// must catch it.
	t.Run("manual decode with preserved headers", func(t *testing.T) {
		runner, req := runnerFor(t, srv.URL+"/")
		req.Headers["Accept-Encoding"] = "gzip"
		resp, err := runner.Execute(&req)
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		if !resp.Cancelled {
			t.Fatalf("expected cancelled response, got %d bytes of data", len(resp.Data))
		}
		if len(resp.Data) > MAX_DOWNLOAD_SIZE {
			t.Fatalf("read %d bytes, must not exceed MAX_DOWNLOAD_SIZE (%d)", len(resp.Data), MAX_DOWNLOAD_SIZE)
		}
	})
}

// A normal, small response must still be read in full and not cancelled, so the
// size cap does not regress ordinary fuzzing behaviour.
func TestExecute_NormalResponseUnaffected(t *testing.T) {
	body := []byte("hello world")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	runner, req := runnerFor(t, srv.URL+"/")
	resp, err := runner.Execute(&req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resp.Cancelled {
		t.Fatal("normal response must not be cancelled")
	}
	if !bytes.Equal(resp.Data, body) {
		t.Fatalf("body = %q, want %q", resp.Data, body)
	}
}

// chunkedBodyServer streams `total` bytes with no Content-Length, so the response
// is framed chunked and the numeric size guard cannot fire.
func chunkedBodyServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	chunk := bytes.Repeat([]byte("A"), 1<<20)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for written := 0; written < total; written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}

func runnerWithConfig(t *testing.T, url string, mutate func(*ffuf.Config)) (ffuf.RunnerProvider, ffuf.Request) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Method = "GET"
	if mutate != nil {
		mutate(&conf)
	}
	runner := NewSimpleRunner(&conf, false)
	req := ffuf.NewRequest(&conf)
	req.Method = "GET"
	req.Url = url
	return runner, req
}

// allocatedBy reports how many bytes were allocated while fn ran.
func allocatedBy(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// The -od and -audit-log snapshot path takes its own copy of the response body,
// and it runs before the LimitReader that bounds resp.Data. Without a bound of its
// own it reads an attacker-chosen amount into memory, so the size cap held for
// resp.Data but not for resp.Raw. Regression test for that: with either output
// feature enabled, a body far over the cap must not be materialised.
func TestExecute_SnapshotIsBoundedWithOutputFeatures(t *testing.T) {
	const body = 64 << 20 // 64 MiB against a 5 MiB cap
	// The ceiling is a multiple of the body, not an absolute figure. The test origin
	// runs in this process and allocates the body it streams, so roughly one body's
	// worth is charged here no matter how little the client reads; the race detector
	// adds more on top. The signal is the multiple: bounded stays near that floor,
	// unbounded allocates several times the body because it buffers, copies and
	// re-serialises it.
	const allocCeiling = 2 * body

	cases := []struct {
		name   string
		mutate func(*ffuf.Config)
	}{
		{"output directory", func(c *ffuf.Config) { c.OutputDirectory = t.TempDir() }},
		{"audit log", func(c *ffuf.Config) { c.AuditLog = filepath.Join(t.TempDir(), "audit.log") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := chunkedBodyServer(t, body)
			defer srv.Close()

			runner, req := runnerWithConfig(t, srv.URL+"/", tc.mutate)
			var resp ffuf.Response
			allocated := allocatedBy(func() {
				var err error
				resp, err = runner.Execute(&req)
				if err != nil {
					t.Fatalf("execute: %v", err)
				}
			})

			if allocated > allocCeiling {
				t.Errorf("allocated %d bytes for a %d byte body, want at most %d; the snapshot is reading past the cap",
					allocated, body, allocCeiling)
			}
			if len(resp.Raw) > MAX_DOWNLOAD_SIZE {
				t.Errorf("resp.Raw is %d bytes, must not exceed MAX_DOWNLOAD_SIZE (%d)", len(resp.Raw), MAX_DOWNLOAD_SIZE)
			}
			if !resp.Cancelled {
				t.Error("expected an over-cap response to be cancelled")
			}
		})
	}
}

// A gzip bomb reaches the same snapshot path: the transport requests gzip itself,
// decompresses transparently and strips Content-Length, so the numeric guard is
// skipped and the snapshot sees the expanded stream.
func TestExecute_SnapshotIsBoundedForGzipBomb(t *testing.T) {
	const decompressed = 64 << 20
	bomb := gzipBomb(t, decompressed)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bomb)
	}))
	defer srv.Close()

	runner, req := runnerWithConfig(t, srv.URL+"/", func(c *ffuf.Config) {
		c.OutputDirectory = t.TempDir()
	})
	resp, err := runner.Execute(&req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(resp.Raw) > MAX_DOWNLOAD_SIZE {
		t.Errorf("resp.Raw is %d bytes, must not exceed MAX_DOWNLOAD_SIZE (%d)", len(resp.Raw), MAX_DOWNLOAD_SIZE)
	}
}

// A response within the cap must still be captured in full, so the size bound does
// not quietly truncate ordinary -od output.
func TestExecute_SnapshotKeptForNormalResponse(t *testing.T) {
	body := []byte("hello world")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	runner, req := runnerWithConfig(t, srv.URL+"/", func(c *ffuf.Config) {
		c.OutputDirectory = t.TempDir()
	})
	resp, err := runner.Execute(&req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resp.Cancelled {
		t.Fatal("a normal response must not be cancelled")
	}
	if !strings.Contains(resp.Raw, string(body)) {
		t.Errorf("resp.Raw does not contain the body: %q", resp.Raw)
	}
}

// -ignore-body used to live inside the Content-Length branch, so it did nothing for
// a chunked response. It must hold regardless of framing.
func TestExecute_IgnoreBodyAppliesToChunkedResponse(t *testing.T) {
	srv := chunkedBodyServer(t, 32<<10)
	defer srv.Close()

	runner, req := runnerWithConfig(t, srv.URL+"/", func(c *ffuf.Config) { c.IgnoreBody = true })
	resp, err := runner.Execute(&req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !resp.Cancelled {
		t.Error("expected the response to be cancelled")
	}
	if len(resp.Data) != 0 {
		t.Errorf("read %d bytes of body despite -ignore-body", len(resp.Data))
	}
}

// Moving the -ignore-body check must not lose the server-declared length, which the
// size filters read.
func TestExecute_IgnoreBodyKeepsContentLength(t *testing.T) {
	body := make([]byte, 32<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	runner, req := runnerWithConfig(t, srv.URL+"/", func(c *ffuf.Config) { c.IgnoreBody = true })
	resp, err := runner.Execute(&req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !resp.Cancelled {
		t.Error("expected the response to be cancelled")
	}
	if resp.ContentLength != int64(len(body)) {
		t.Errorf("ContentLength = %d, want %d", resp.ContentLength, len(body))
	}
}

// Response headers sit outside MAX_DOWNLOAD_SIZE, so the transport needs its own
// ceiling; Go's default is 10 MB.
func TestNewSimpleRunner_BoundsResponseHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := ffuf.NewConfig(ctx, cancel)
	runner := NewSimpleRunner(&conf, false).(*SimpleRunner)

	transport, ok := runner.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", runner.client.Transport)
	}
	if transport.MaxResponseHeaderBytes <= 0 {
		t.Fatal("MaxResponseHeaderBytes is unset, response headers are bounded only by the net/http default")
	}
	if transport.MaxResponseHeaderBytes > 4<<20 {
		t.Errorf("MaxResponseHeaderBytes = %d, larger than intended", transport.MaxResponseHeaderBytes)
	}
}

// prepareCase is one SimpleRunner.Prepare input and the request it must produce.
// wantHeaders are the canonical names and the substituted values.
type prepareCase struct {
	name        string
	method      string
	url         string
	body        string
	headers     map[string]string
	input       map[string][]byte
	wantMethod  string
	wantURL     string
	wantBody    string
	wantHeaders map[string]string
}

// Issue #885: wordlists named FUZZ and FUZZ1 were applied with strings.ReplaceAll
// in map iteration order. Replacing FUZZ first turned /FUZZ/FUZZ1 into /ABC/ABC1,
// and the replay runner prepares the same input on its own, so the two requests
// could disagree. Longer keywords win, and each field is substituted once.
func TestPrepare_OverlappingKeywords(t *testing.T) {
	runner, _ := runnerFor(t, "http://unused.example/")

	cases := []prepareCase{
		{
			name:        "fuzz_and_fuzz1",
			method:      "POST",
			url:         "http://example.test/FUZZ/FUZZ1",
			body:        "a=FUZZ&b=FUZZ1",
			headers:     map[string]string{"X-Fuzz": "FUZZ", "X-Fuzz1": "FUZZ1"},
			input:       map[string][]byte{"FUZZ": []byte("ABC"), "FUZZ1": []byte("XYZ")},
			wantMethod:  "POST",
			wantURL:     "http://example.test/ABC/XYZ",
			wantBody:    "a=ABC&b=XYZ",
			wantHeaders: map[string]string{"X-Fuzz": "ABC", "X-Fuzz1": "XYZ"},
		},
		{
			name:    "three_key_prefix_chain",
			method:  "FUZZ",
			url:     "http://example.test/FUZZ/FUZZ1/FUZZ12",
			body:    "FUZZ|FUZZ1|FUZZ12",
			headers: map[string]string{"X-Chain": "FUZZ-FUZZ1-FUZZ12"},
			input: map[string][]byte{
				"FUZZ":   []byte("ABC"),
				"FUZZ1":  []byte("XYZ"),
				"FUZZ12": []byte("QQ"),
			},
			wantMethod:  "ABC",
			wantURL:     "http://example.test/ABC/XYZ/QQ",
			wantBody:    "ABC|XYZ|QQ",
			wantHeaders: map[string]string{"X-Chain": "ABC-XYZ-QQ"},
		},
		{
			name:        "repeated_and_adjacent",
			method:      "GET",
			url:         "http://example.test/FUZZ/FUZZ/FUZZ1FUZZ/FUZZ1",
			body:        "FUZZFUZZ FUZZ1FUZZ1",
			headers:     map[string]string{"X-Rep": "FUZZFUZZ1FUZZ"},
			input:       map[string][]byte{"FUZZ": []byte("ABC"), "FUZZ1": []byte("XYZ")},
			wantMethod:  "GET",
			wantURL:     "http://example.test/ABC/ABC/XYZABC/XYZ",
			wantBody:    "ABCABC XYZXYZ",
			wantHeaders: map[string]string{"X-Rep": "ABCXYZABC"},
		},
		{
			name:        "unrelated_custom_names",
			method:      "PUT",
			url:         "http://example.test/USER/PASS",
			body:        "user=USER&pass=PASS",
			headers:     map[string]string{"X-User": "USER", "X-Pass": "PASS"},
			input:       map[string][]byte{"USER": []byte("alice"), "PASS": []byte("s3cret")},
			wantMethod:  "PUT",
			wantURL:     "http://example.test/alice/s3cret",
			wantBody:    "user=alice&pass=s3cret",
			wantHeaders: map[string]string{"X-User": "alice", "X-Pass": "s3cret"},
		},
		{
			// "word" is a lowercase prefix of "wordlist". The header name is
			// substituted before it is canonicalized, so the lowercase keyword
			// still matches, and the longer keyword wins where both could.
			name:   "lowercase_header_keyword",
			method: "GET",
			url:    "http://example.test/word/wordlist",
			body:   "k=word&list=wordlist",
			headers: map[string]string{
				"x-wordlist-id": "word",
				"x-word-token":  "wordlist",
			},
			input:      map[string][]byte{"word": []byte("alpha"), "wordlist": []byte("beta")},
			wantMethod: "GET",
			wantURL:    "http://example.test/alpha/beta",
			wantBody:   "k=alpha&list=beta",
			wantHeaders: map[string]string{
				"X-Beta-Id":     "alpha",
				"X-Alpha-Token": "beta",
			},
		},
		{
			name:        "empty_payload",
			method:      "POST",
			url:         "http://example.test/preFUZZpost/FUZZ1",
			body:        "[FUZZ]",
			headers:     map[string]string{"x-empty": "FUZZ", "x-plain": "stay"},
			input:       map[string][]byte{"FUZZ": []byte(""), "FUZZ1": []byte("XYZ")},
			wantMethod:  "POST",
			wantURL:     "http://example.test/prepost/XYZ",
			wantBody:    "[]",
			wantHeaders: map[string]string{"X-Empty": "", "X-Plain": "stay"},
		},
		{
			name:        "single_keyword",
			method:      "GET",
			url:         "http://example.test/FUZZ/FUZZ",
			body:        "FUZZ",
			headers:     map[string]string{"X-Single": "FUZZ"},
			input:       map[string][]byte{"FUZZ": []byte("ABC")},
			wantMethod:  "GET",
			wantURL:     "http://example.test/ABC/ABC",
			wantBody:    "ABC",
			wantHeaders: map[string]string{"X-Single": "ABC"},
		},
		{
			name:        "no_occurrences",
			method:      "GET",
			url:         "http://example.test/static",
			body:        "nothing",
			headers:     map[string]string{"x-untouched": "still"},
			input:       map[string][]byte{"FUZZ": []byte("ABC")},
			wantMethod:  "GET",
			wantURL:     "http://example.test/static",
			wantBody:    "nothing",
			wantHeaders: map[string]string{"X-Untouched": "still"},
		},
		{
			// Nothing to substitute, so the copied header name stays as written.
			name:        "empty_input",
			method:      "GET",
			url:         "http://example.test/static",
			body:        "nothing",
			headers:     map[string]string{"x-untouched": "still"},
			input:       map[string][]byte{},
			wantMethod:  "GET",
			wantURL:     "http://example.test/static",
			wantBody:    "nothing",
			wantHeaders: map[string]string{"x-untouched": "still"},
		},
		{
			name:        "nil_input",
			method:      "GET",
			url:         "http://example.test/static",
			body:        "nothing",
			headers:     map[string]string{"x-untouched": "still"},
			input:       nil,
			wantMethod:  "GET",
			wantURL:     "http://example.test/static",
			wantBody:    "nothing",
			wantHeaders: map[string]string{"x-untouched": "still"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, keyed := range prepareInputs(tc.input) {
				t.Run(keyed.label, func(t *testing.T) {
					exercisePrepare(t, runner, tc, keyed.input)
				})
			}
		})
	}
}

// Payloads that contain a keyword must be inserted literally. Sequential
// ReplaceAll rescans the inserted text, and for mutually referring payloads
// every map order produces a different request than one pass: FUZZ -> FUZZ1
// then FUZZ1 -> FUZZ turns /FUZZ/FUZZ1 into either /FUZZ/FUZZ1 or /FUZZ1/FUZZ1,
// never /FUZZ1/FUZZ.
func TestPrepare_InsertedPayloadsStayLiteral(t *testing.T) {
	runner, _ := runnerFor(t, "http://unused.example/")

	cases := []prepareCase{
		{
			name:        "value_contains_other_keyword",
			method:      "POST",
			url:         "http://example.test/FUZZ/end",
			body:        "FUZZ",
			headers:     map[string]string{"X-In": "FUZZ", "x-out": "tail"},
			input:       map[string][]byte{"FUZZ": []byte("xxFUZZ1yy"), "FUZZ1": []byte("XYZ")},
			wantMethod:  "POST",
			wantURL:     "http://example.test/xxFUZZ1yy/end",
			wantBody:    "xxFUZZ1yy",
			wantHeaders: map[string]string{"X-In": "xxFUZZ1yy", "X-Out": "tail"},
		},
		{
			name:       "mutually_referring_payloads",
			method:     "FUZZ",
			url:        "http://example.test/FUZZ/FUZZ1",
			body:       "FUZZ|FUZZ1",
			headers:    map[string]string{"X-FUZZ-Id": "FUZZ1", "X-Val": "FUZZ"},
			input:      map[string][]byte{"FUZZ": []byte("FUZZ1"), "FUZZ1": []byte("FUZZ")},
			wantMethod: "FUZZ1",
			wantURL:    "http://example.test/FUZZ1/FUZZ",
			wantBody:   "FUZZ1|FUZZ",
			wantHeaders: map[string]string{
				"X-Fuzz1-Id": "FUZZ",
				"X-Val":      "FUZZ1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				input := rotatedInput(tc.input, i)
				exercisePrepare(t, runner, tc, input)
			}
		})
	}
}

// The normal runner and the replay runner prepare the same template
// independently, then send it: the target directly, and the replay proxy as an
// HTTP proxy that records the request. Both must observe /ABC/XYZ and the same
// substituted headers and body.
func TestPrepare_NormalAndReplaySendTheSameRequest(t *testing.T) {
	targetSink := &requestSink{}
	proxySink := &requestSink{}
	target := httptest.NewServer(targetSink.handler())
	defer target.Close()
	proxy := httptest.NewServer(proxySink.handler())
	defer proxy.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Timeout = 5
	conf.ReplayProxyURL = proxy.URL
	normal := NewSimpleRunner(&conf, false)
	replay := NewSimpleRunner(&conf, true)

	base := ffuf.Request{
		Method: "POST",
		Url:    target.URL + "/FUZZ/FUZZ1",
		Data:   []byte("a=FUZZ&b=FUZZ1&c=fuzz"),
		Headers: map[string]string{
			"User-Agent":    "ffuf-replay-test",
			"X-Token":       "FUZZ",
			"X-Fuzz1":       "FUZZ1",
			"x-fuzz-bucket": "fuzz",
		},
		Position: 4,
		Host:     "keep.example",
		Raw:      "keep-raw",
	}
	input := map[string][]byte{
		"FUZZ":  []byte("ABC"),
		"FUZZ1": []byte("XYZ"),
		"fuzz":  []byte("qq"),
	}

	baseNormal := ffuf.CopyRequest(&base)
	baseReplay := ffuf.CopyRequest(&base)
	inputNormal := cloneInput(input)
	inputReplay := cloneInput(input)
	normalBefore := snapshotRequest(baseNormal)
	replayBefore := snapshotRequest(baseReplay)
	normalInputBefore := inputSnapshot(inputNormal)
	replayInputBefore := inputSnapshot(inputReplay)

	normalReq, err := normal.Prepare(inputNormal, &baseNormal)
	if err != nil {
		t.Fatalf("normal Prepare: %v", err)
	}
	replayReq, err := replay.Prepare(inputReplay, &baseReplay)
	if err != nil {
		t.Fatalf("replay Prepare: %v", err)
	}

	wantURL := target.URL + "/ABC/XYZ"
	wantBody := "a=ABC&b=XYZ&c=qq"
	wantHeaders := map[string]string{
		"User-Agent":  "ffuf-replay-test",
		"X-Token":     "ABC",
		"X-Fuzz1":     "XYZ",
		"X-Qq-Bucket": "qq",
	}
	for _, got := range []ffuf.Request{normalReq, replayReq} {
		if got.Method != "POST" {
			t.Fatalf("method = %q, want POST", got.Method)
		}
		if got.Url != wantURL {
			t.Fatalf("url = %q, want %q", got.Url, wantURL)
		}
		if string(got.Data) != wantBody {
			t.Fatalf("body = %q, want %q", got.Data, wantBody)
		}
		if !sameStringMap(got.Headers, wantHeaders) {
			t.Fatalf("headers = %s, want %s", formatHeaders(got.Headers), formatHeaders(wantHeaders))
		}
	}
	if normalReq.Method != replayReq.Method || normalReq.Url != replayReq.Url || string(normalReq.Data) != string(replayReq.Data) || !sameStringMap(normalReq.Headers, replayReq.Headers) {
		t.Fatalf("normal and replay prepared different requests: %+v vs %+v", normalReq, replayReq)
	}
	assertRequestUnchanged(t, baseNormal, normalBefore)
	assertRequestUnchanged(t, baseReplay, replayBefore)
	assertInputUnchanged(t, inputNormal, normalInputBefore)
	assertInputUnchanged(t, inputReplay, replayInputBefore)

	if _, err := normal.Execute(&normalReq); err != nil {
		t.Fatalf("normal Execute: %v", err)
	}
	if _, err := replay.Execute(&replayReq); err != nil {
		t.Fatalf("replay Execute: %v", err)
	}

	targetSeen := targetSink.requests()
	proxySeen := proxySink.requests()
	if len(targetSeen) != 1 {
		t.Fatalf("target saw %d requests, want 1", len(targetSeen))
	}
	if len(proxySeen) != 1 {
		t.Fatalf("replay proxy saw %d requests, want 1 (request URI %q)", len(proxySeen), requestURIs(proxySeen))
	}
	assertObservedRequest(t, "target", targetSeen[0], "POST", "/ABC/XYZ", wantBody, wantHeaders)
	assertObservedRequest(t, "replay proxy", proxySeen[0], "POST", "/ABC/XYZ", wantBody, wantHeaders)
	if proxySeen[0].host != targetSeen[0].host {
		t.Fatalf("replay host = %q, target host = %q", proxySeen[0].host, targetSeen[0].host)
	}
	for _, key := range []string{"User-Agent", "X-Token", "X-Fuzz1", "X-Qq-Bucket"} {
		if proxySeen[0].header.Get(key) != targetSeen[0].header.Get(key) {
			t.Fatalf("header %s: proxy %q, target %q", key, proxySeen[0].header.Get(key), targetSeen[0].header.Get(key))
		}
	}
}

// preservedPosition, preservedHost and preservedRaw ride along on the base
// request. Prepare copies them and does not substitute into them.
const (
	preservedPosition = 11
	preservedHost     = "keep.example"
	preservedRaw      = "keep-raw"
)

type keyedInput struct {
	label string
	input map[string][]byte
}

type requestSnap struct {
	method   string
	url      string
	host     string
	raw      string
	position int
	body     string
	headers  map[string]string
}

type seenRequest struct {
	method     string
	path       string
	host       string
	body       string
	requestURI string
	header     http.Header
}

type requestSink struct {
	mu  sync.Mutex
	got []seenRequest
}

func (s *requestSink) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		s.got = append(s.got, seenRequest{
			method:     r.Method,
			path:       r.URL.Path,
			host:       r.Host,
			body:       string(body),
			requestURI: r.RequestURI,
			header:     r.Header.Clone(),
		})
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func (s *requestSink) requests() []seenRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]seenRequest, len(s.got))
	copy(out, s.got)
	return out
}

func prepareInputs(payloads map[string][]byte) []keyedInput {
	if payloads == nil {
		return []keyedInput{{label: "nil", input: nil}}
	}
	keys := make([]string, 0, len(payloads))
	for k := range payloads {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	orders := permuteStrings(keys)
	out := make([]keyedInput, 0, len(orders))
	for _, order := range orders {
		label := strings.Join(order, ",")
		if label == "" {
			label = "empty"
		}
		out = append(out, keyedInput{label: label, input: inputFromOrder(payloads, order)})
	}
	return out
}

func permuteStrings(keys []string) [][]string {
	if len(keys) == 0 {
		return [][]string{nil}
	}
	var out [][]string
	var rec func(remain, acc []string)
	rec = func(remain, acc []string) {
		if len(remain) == 0 {
			out = append(out, append([]string(nil), acc...))
			return
		}
		for i := range remain {
			next := append(append([]string(nil), remain[:i]...), remain[i+1:]...)
			rec(next, append(acc, remain[i]))
		}
	}
	rec(append([]string(nil), keys...), nil)
	return out
}

func inputFromOrder(payloads map[string][]byte, order []string) map[string][]byte {
	in := make(map[string][]byte, len(order))
	for _, k := range order {
		in[k] = append([]byte(nil), payloads[k]...)
	}
	return in
}

func rotatedInput(payloads map[string][]byte, shift int) map[string][]byte {
	keys := make([]string, 0, len(payloads))
	for k := range payloads {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return map[string][]byte{}
	}
	shift %= len(keys)
	keys = append(keys[shift:], keys[:shift]...)
	if shift%2 == 1 {
		for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
			keys[i], keys[j] = keys[j], keys[i]
		}
	}
	return inputFromOrder(payloads, keys)
}

func cloneInput(in map[string][]byte) map[string][]byte {
	if in == nil {
		return nil
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		out[k] = append([]byte(nil), v...)
	}
	return out
}

func cloneHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

func newBase(tc prepareCase) ffuf.Request {
	return ffuf.Request{
		Method:   tc.method,
		Host:     preservedHost,
		Url:      tc.url,
		Headers:  cloneHeaders(tc.headers),
		Data:     []byte(tc.body),
		Position: preservedPosition,
		Raw:      preservedRaw,
	}
}

func snapshotRequest(req ffuf.Request) requestSnap {
	return requestSnap{
		method:   req.Method,
		url:      req.Url,
		host:     req.Host,
		raw:      req.Raw,
		position: req.Position,
		body:     string(req.Data),
		headers:  cloneHeaders(req.Headers),
	}
}

func inputSnapshot(in map[string][]byte) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}

func exercisePrepare(t *testing.T, runner ffuf.RunnerProvider, tc prepareCase, input map[string][]byte) {
	t.Helper()
	base := newBase(tc)
	baseBefore := snapshotRequest(base)
	inputBefore := inputSnapshot(input)

	var prev ffuf.Request
	for call := 0; call < 2; call++ {
		got, err := runner.Prepare(input, &base)
		if err != nil {
			t.Fatalf("Prepare call %d: %v", call, err)
		}
		assertPreparedMatches(t, got, tc)
		assertInputMetadata(t, got, input)
		if call == 1 {
			assertSamePrepared(t, prev, got)
		}
		prev = got
	}
	assertRequestUnchanged(t, base, baseBefore)
	assertInputUnchanged(t, input, inputBefore)
}

func assertPreparedMatches(t *testing.T, got ffuf.Request, tc prepareCase) {
	t.Helper()
	if got.Method != tc.wantMethod {
		t.Fatalf("method = %q, want %q", got.Method, tc.wantMethod)
	}
	if got.Url != tc.wantURL {
		t.Fatalf("url = %q, want %q", got.Url, tc.wantURL)
	}
	if string(got.Data) != tc.wantBody {
		t.Fatalf("body = %q, want %q", got.Data, tc.wantBody)
	}
	if !sameStringMap(got.Headers, tc.wantHeaders) {
		t.Fatalf("headers = %s, want %s", formatHeaders(got.Headers), formatHeaders(tc.wantHeaders))
	}
	if got.Position != preservedPosition || got.Host != preservedHost || got.Raw != preservedRaw {
		t.Fatalf("copied fields = position %d host %q raw %q, want position %d host %q raw %q",
			got.Position, got.Host, got.Raw, preservedPosition, preservedHost, preservedRaw)
	}
}

func assertInputMetadata(t *testing.T, got ffuf.Request, input map[string][]byte) {
	t.Helper()
	if input == nil {
		if got.Input != nil {
			t.Fatalf("Input = %#v, want nil", got.Input)
		}
		return
	}
	if len(got.Input) != len(input) {
		t.Fatalf("Input has %d keys, want %d", len(got.Input), len(input))
	}
	for k, v := range input {
		if !bytes.Equal(got.Input[k], v) {
			t.Fatalf("Input[%s] = %q, want %q", k, got.Input[k], v)
		}
	}
}

func assertSamePrepared(t *testing.T, a, b ffuf.Request) {
	t.Helper()
	if a.Method != b.Method || a.Url != b.Url || string(a.Data) != string(b.Data) || a.Position != b.Position || a.Host != b.Host || a.Raw != b.Raw || !sameStringMap(a.Headers, b.Headers) {
		t.Fatalf("repeated Prepare disagreed: method %q/%q url %q/%q body %q/%q headers %s vs %s",
			a.Method, b.Method, a.Url, b.Url, a.Data, b.Data, formatHeaders(a.Headers), formatHeaders(b.Headers))
	}
}

func assertRequestUnchanged(t *testing.T, req ffuf.Request, before requestSnap) {
	t.Helper()
	if req.Method != before.method || req.Url != before.url || req.Host != before.host || req.Raw != before.raw || req.Position != before.position || string(req.Data) != before.body || !sameStringMap(req.Headers, before.headers) {
		t.Fatalf("base request changed: method %q url %q body %q headers %s",
			req.Method, req.Url, req.Data, formatHeaders(req.Headers))
	}
}

func assertInputUnchanged(t *testing.T, input map[string][]byte, before map[string]string) {
	t.Helper()
	if !sameStringMap(inputSnapshot(input), before) {
		t.Fatalf("input changed: %s, want %s", formatHeaders(inputSnapshot(input)), formatHeaders(before))
	}
}

func assertObservedRequest(t *testing.T, who string, got seenRequest, method, path, body string, headers map[string]string) {
	t.Helper()
	if got.method != method {
		t.Fatalf("%s method = %q, want %q (request URI %q)", who, got.method, method, got.requestURI)
	}
	if got.path != path {
		t.Fatalf("%s path = %q, want %q (request URI %q)", who, got.path, path, got.requestURI)
	}
	if got.body != body {
		t.Fatalf("%s body = %q, want %q", who, got.body, body)
	}
	for k, v := range headers {
		if got.header.Get(k) != v {
			t.Fatalf("%s header %s = %q, want %q", who, k, got.header.Get(k), v)
		}
	}
}

func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func formatHeaders(h map[string]string) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s=%q", k, h[k])
	}
	return b.String()
}

func requestURIs(reqs []seenRequest) string {
	parts := make([]string, len(reqs))
	for i, req := range reqs {
		parts[i] = req.requestURI
	}
	return strings.Join(parts, ", ")
}
