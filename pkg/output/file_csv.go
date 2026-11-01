package output

import (
	"encoding/base64"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

var staticheaders = []string{"url", "redirectlocation", "position", "status_code", "content_length", "content_words", "content_lines", "content_type", "duration", "resultfile", "Ffufhash"}

func writeCSV(filename string, config *ffuf.Config, res []ffuf.Result, encode bool) error {
	header := make([]string, 0)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	keywords := make([]string, 0, len(config.InputProviders))
	for _, inputprovider := range config.InputProviders {
		keywords = append(keywords, inputprovider.Keyword)
	}
	header = append(header, keywords...)
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

		err := w.Write(toCSV(r, keywords))
		if err != nil {
			return err
		}
	}
	return nil
}

func base64encode(in []byte) string {
	return base64.StdEncoding.EncodeToString(in)
}

// toCSV converts a result into a CSV row. The input values are written in the
// order of keywords, so that each value ends up under its own header column
// regardless of the (random) iteration order of the r.Input map.
func toCSV(r ffuf.Result, keywords []string) []string {
	res := make([]string, 0)
	for _, keyword := range keywords {
		res = append(res, string(r.Input[keyword]))
	}
	ffufhash := string(r.Input["FFUFHASH"])
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
