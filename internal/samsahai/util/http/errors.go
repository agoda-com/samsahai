package http

import (
	"errors"
	"fmt"
	"net/http"
)

func Error(errCode int) error {
	return errors.New(fmt.Sprintf("error status code %d - %s", errCode, http.StatusText(errCode)))
}
