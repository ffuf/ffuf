package output

import (
	"github.com/ffuf/ffuf/pkg/config"
)

func NewOutputProviderByName(name string, conf *config.Config) OutputProvider {
	//We have only one outputprovider at the moment
	return NewStdoutput(conf)
}
