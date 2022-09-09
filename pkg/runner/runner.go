package runner

import (
	"github.com/ffuf/ffuf/pkg/config"
)

func NewRunnerByName(name string, conf *config.Config, replay bool) RunnerProvider {
	// We have only one Runner at the moment
	return NewSimpleRunner(conf, replay)
}
