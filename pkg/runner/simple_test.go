package runner

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
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
