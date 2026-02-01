package runner
import (
	"compress/gzip"
	"compress/flate"
	"io"
	"net/http"
	"github.com/andybalholm/brotli"
)
// WrapBody ensures the reader always provides decompressed data if Transfer-Encoding is set
func WrapBody(resp *http.Response) (io.ReadCloser, error) {
    switch resp.Header.Get("Content-Encoding") {
    case "gzip":
        return gzip.NewReader(resp.Body)
    case "br":
        return io.NopCloser(brotli.NewReader(resp.Body)), nil
    case "deflate":
        return flate.NewReader(resp.Body), nil
    default:
        return resp.Body, nil
    }
}
