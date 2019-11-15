package function

import (
	"errors"
)

var (
	errNoConfigurationFile = errors.New("no configuration file exists in the source repository")
)
