package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	// Generous ceiling: the bounded path allocates a small multiple of the cap,
	// the unbounded one allocates several times the body.
	const allocCeiling = 64 << 20

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
				t.Errorf("allocated %d bytes for a %d byte body, want at most %d", allocated, body, allocCeiling)
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
