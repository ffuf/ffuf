package output

import (
	"encoding/base64"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

var staticheaders = []string{"url", "redirectlocation", "position", "status_code", "content_length", "content_words", "content_lines", "content_type", "duration", "resultfile", "Ffufhash"}

func writeCSV(filename string, config *ffuf.Config, res []ffuf.Result, encode bool) (err error) {
	header := make([]string, 0)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	// Flush before Close, and surface both. csv.Writer buffers, so a write
	// error (a full disk, say) is only observable via w.Error() after the
	// flush -- previously both this and the Close error were discarded.
	defer func() {
		w.Flush()
		if werr := w.Error(); err == nil {
			err = werr
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	for _, inputprovider := range config.InputProviders {
		header = append(header, inputprovider.Keyword)
	}
	header = append(header, staticheaders...)

	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range res {
		if encode {
			inputs := make(map[string][]byte, len(r.Input))
			for k, v := range r.Input {
				inputs[k] = []byte(base64encode(v))
			}
			r.Input = inputs
		}

		err := w.Write(toCSV(r))
		if err != nil {
			return err
		}
	}
	return nil
}

func base64encode(in []byte) string {
	return base64.StdEncoding.EncodeToString(in)
}

func toCSV(r ffuf.Result) []string {
	res := make([]string, 0)
	ffufhash := ""
	for k, v := range r.Input {
		if k == "FFUFHASH" {
			ffufhash = string(v)
		} else {
			res = append(res, string(v))
		}
	}
	res = append(res, r.Url)
	res = append(res, r.RedirectLocation)
	res = append(res, strconv.Itoa(r.Position))
	res = append(res, strconv.FormatInt(r.StatusCode, 10))
	res = append(res, strconv.FormatInt(r.ContentLength, 10))
	res = append(res, strconv.FormatInt(r.ContentWords, 10))
	res = append(res, strconv.FormatInt(r.ContentLines, 10))
	res = append(res, r.ContentType)
	res = append(res, r.Duration.String())
	res = append(res, r.ResultFile)
	res = append(res, ffufhash)
	return res
}
