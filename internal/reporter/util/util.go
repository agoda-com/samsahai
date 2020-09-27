package util

import (
	"strings"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

const (
	statusSuccess = "success"
	statusFailure = "failure"
)

func CheckMatchingInterval(interval s2hv1beta1.ReporterInterval, isReverify bool) error {
	switch interval {
	case s2hv1beta1.IntervalEveryTime:
	default:
		if !isReverify {
			return s2herrors.New("interval was not matched")
		}
	}

	return nil
}

func CheckMatchingCriteria(criteria s2hv1beta1.ReporterCriteria, result string) error {
	lowerCaseResult := strings.ToLower(result)

	switch criteria {
	case s2hv1beta1.CriteriaBoth:
	case s2hv1beta1.CriteriaSuccess:
		if lowerCaseResult != statusSuccess {
			return s2herrors.New("criteria was not matched")
		}
	default:
		if lowerCaseResult != statusFailure {
			return s2herrors.New("criteria was not matched")
		}
	}

	return nil
}
