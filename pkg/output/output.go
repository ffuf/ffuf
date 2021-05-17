package output

import (
	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewOutputProviderByName(name string, conf *ffuf.Config) ffuf.OutputProvider {
	//We have only one outputprovider at the moment
	return NewStdoutput(conf)
}

func formatFileName(config *ffuf.Config, filename string, extension string) string {
	if config.AutoName || config.OutputFormat == "all" {
		// Only add extension if AutoName is enabled or every format is printed
		return filename + extension
	} else {
		// Do not add extension if single files are created and AutoName is disabled
		return filename
	}
}
