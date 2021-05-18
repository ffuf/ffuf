package output

import (
	"fmt"
	"os"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewOutputProviderByName(name string, conf *ffuf.Config) ffuf.OutputProvider {
	//We have only one outputprovider at the moment
	return NewStdoutput(conf)
}

func buildFilename(config *ffuf.Config, filename string, extension string) string {
	if config.AutoName {
		if _, err := os.Stat(filename + extension); os.IsNotExist(err) {
			// AutoName is used and file does not exist
			return filename + extension
		}
		// AutoName is used and file already exists -> add timestamp
		t := time.Now()
		return fmt.Sprintf("%s_%d%02d%02d_%02d%02d%02d%s", filename, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), extension)
	}
	if config.OutputFormat == "all" {
		// Add extension if all formats are printed
		return filename + extension
	}
	// AutoName is not used and only a single output format is specified
	return filename
}
