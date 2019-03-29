package http

import (
	"fmt"
	"net/http"
)

func Error(errCode int) error {
	return fmt.Errorf("error status code %d - %s", errCode, http.StatusText(errCode))
}
