package rpc

import (
	"fmt"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal/errors"
)

func (x *PullRequestTearDownDuration_Criteria) FromCrdCriteria(criteria s2hv1.PullRequestTearDownDurationCriteria) (err error) {
	switch criteria {
	case s2hv1.PullRequestTearDownDurationCriteriaBoth:
		*x = PullRequestTearDownDuration_Criteria_BOTH
	case s2hv1.PullRequestTearDownDurationCriteriaFailure:
		*x = PullRequestTearDownDuration_Criteria_FAILURE
	case s2hv1.PullRequestTearDownDurationCriteriaSuccess:
		*x = PullRequestTearDownDuration_Criteria_SUCCESS
	default:
		err = fmt.Errorf("%v is not a valid tearDownDuration criteria", criteria)
	}
	return
}

func (x *PullRequestTearDownDuration_Criteria) ToCrdCriteria() (criteria s2hv1.PullRequestTearDownDurationCriteria, err error) {
	switch *x {
	case PullRequestTearDownDuration_Criteria_UNKNOWN:
		err = errors.ErrPullRequestRPCTearDownDurationCriteriaUnknown
	case PullRequestTearDownDuration_Criteria_BOTH:
		criteria = s2hv1.PullRequestTearDownDurationCriteriaBoth
	case PullRequestTearDownDuration_Criteria_FAILURE:
		criteria = s2hv1.PullRequestTearDownDurationCriteriaFailure
	case PullRequestTearDownDuration_Criteria_SUCCESS:
		criteria = s2hv1.PullRequestTearDownDurationCriteriaSuccess
	default:
		err = fmt.Errorf("%v is not a valid tearDownDuration criteria", *x)
	}
	return
}
