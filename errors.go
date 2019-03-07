package milter

import (
	"errors"
)

// pre-defined errors
var (
	errCloseSession = errors.New("Stop current milter processing")
)
