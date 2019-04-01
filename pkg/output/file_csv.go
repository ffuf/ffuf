package output

import (
	"encoding/base64"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

var header = []string{"input", "status_code", "content_length", "content_words"}

func writeCSV(config *ffuf.Config, res []Result, encode bool) error {
	f, err := os.Create(config.OutputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range res {
		if encode {
			r.Input = base64encode(r.Input)
		}

		err := w.Write(toCSV(r))
		if err != nil {
			return err
		}
	}
	return nil
}

func base64encode(in string) string {
	return base64.StdEncoding.EncodeToString([]byte(in))
}

func toCSV(r Result) []string {
	return []string{
		r.Input,
		strconv.FormatInt(r.StatusCode, 10),
		strconv.FormatInt(r.ContentLength, 10),
		strconv.FormatInt(r.ContentWords, 10),
	}
}
