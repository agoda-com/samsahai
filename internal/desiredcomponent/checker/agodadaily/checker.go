package agodadaily

import (
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/desiredcomponent/checker/harbor"
)

const (
	CheckerName = "agoda-daily"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second
)

type checker struct {
	harberChecker internal.DesiredComponentChecker
	log           logr.Logger
}

func New() internal.DesiredComponentChecker {
	return &checker{
		harberChecker: harbor.New(),
		log:           log.Log.WithName(CheckerName),
	}
}

func (c *checker) GetName() string {
	return CheckerName
}

func (c *checker) GetVersion(repository, name, pattern string) (string, error) {
	if pattern == "" {
		loc, _ := time.LoadLocation("Asia/Bangkok")
		now := time.Now().In(loc)
		pattern = now.Format("2006\\.01") + "\\.(\\d+)(\\.\\d+)?"
	}
	return c.harberChecker.GetVersion(repository, name, pattern)
}
