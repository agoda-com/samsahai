package internal

import (
	"fmt"
)

var (
	ErrInternalError  = fmt.Errorf("internal error")
	ErrNotImplemented = fmt.Errorf("not implemented")

	ErrRequestTimeout = fmt.Errorf("request timeout")

	ErrImageVersionNotFound = fmt.Errorf("image version not found")

	ErrNoDesiredComponentVersion = fmt.Errorf("no desired component version")
)
