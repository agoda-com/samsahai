package outdated

import (
	"strings"
	"time"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/stringutils"
)

var logger = s2hlog.S2HLog.WithName("Outdated-util")

type Outdated struct {
	cfg                   *s2hv1.ConfigSpec
	desiredCompsImageTime map[string]s2hv1.ImageCreatedTime
	currentActiveComps    map[string]s2hv1.StableComponent
	nowTime               time.Time
}

func New(cfg *s2hv1.ConfigSpec,
	desiredComps map[string]s2hv1.ImageCreatedTime,
	currentActiveComps map[string]s2hv1.StableComponent,
) *Outdated {
	r := &Outdated{
		cfg:                   cfg,
		desiredCompsImageTime: desiredComps,
		currentActiveComps:    currentActiveComps,
		nowTime:               time.Now(),
	}

	return r
}

func (o Outdated) SetOutdatedDuration(atpCompStatus *s2hv1.ActivePromotionStatus) {
	if atpCompStatus.OutdatedComponents == nil {
		atpCompStatus.OutdatedComponents = make(map[string]s2hv1.OutdatedComponent)
	}

	for _, activeComp := range o.currentActiveComps {
		stableCompSpec := activeComp.Spec
		stableName := stableCompSpec.Name
		stableImage := stringutils.ConcatImageString(stableCompSpec.Repository, stableCompSpec.Version)
		desiredCompImageCreatedTime := o.desiredCompsImageTime[stableName].ImageCreatedTime
		if len(desiredCompImageCreatedTime) == 0 {
			logger.Info("no desired component created time list")
			continue
		}

		descCreatedTime := s2hv1.SortByCreatedTimeDESC(desiredCompImageCreatedTime)
		latestDesiredImage := descCreatedTime[0].Image
		latestDesiredImageTime := descCreatedTime[0].ImageTime
		if strings.EqualFold(latestDesiredImage, stableImage) {
			outdatedComp := getOutdatedComponent(stableCompSpec, latestDesiredImageTime, 0)
			atpCompStatus.OutdatedComponents[stableName] = outdatedComp
			continue
		}

		for i := 1; i < len(descCreatedTime); i++ {
			if !strings.EqualFold(descCreatedTime[i].Image, stableImage) {
				continue
			}

			nextAtpStableDesiredTime := descCreatedTime[i-1].ImageTime.CreatedTime
			lastAtpStableDesiredTime := nextAtpStableDesiredTime.Add(-1 * time.Minute)
			outdatedDuration := o.calculateOutdatedDuration(lastAtpStableDesiredTime)
			if o.isExceedOutdatedDuration(outdatedDuration) {
				atpCompStatus.HasOutdatedComponent = true
			} else {
				outdatedDuration = 0
			}

			outdatedComp := getOutdatedComponent(activeComp.Spec, latestDesiredImageTime, outdatedDuration)
			atpCompStatus.OutdatedComponents[stableName] = outdatedComp
		}
	}
}

func (o Outdated) isExceedOutdatedDuration(outdatedDuration time.Duration) bool {
	atpCfg := o.cfg.ActivePromotion
	if atpCfg == nil || atpCfg.OutdatedNotification == nil {
		return false
	}

	exceedDurationCfg := atpCfg.OutdatedNotification.ExceedDuration.Duration
	return outdatedDuration > exceedDurationCfg
}

func (o Outdated) calculateOutdatedDuration(atpStableDesiredTime time.Time) time.Duration {
	totalOutdatedDuration := o.nowTime.Sub(atpStableDesiredTime)
	totalWeekendDuration := o.getWeekendDuration(atpStableDesiredTime)
	totalOutdatedDuration = totalOutdatedDuration - totalWeekendDuration
	return totalOutdatedDuration.Round(time.Minute)
}

func (o Outdated) getWeekendDuration(atpStableDesiredTime time.Time) time.Duration {
	atpCfg := o.cfg.ActivePromotion
	if atpCfg == nil || atpCfg.OutdatedNotification == nil || !atpCfg.OutdatedNotification.ExcludeWeekendCalculation {
		return time.Duration(0)
	}

	fromTime := atpStableDesiredTime
	toTime := o.nowTime
	toTimeEndOfDay := time.Date(toTime.Year(), toTime.Month(), toTime.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	totalWeekendDuration := time.Duration(0)
	for fromTime.Before(toTimeEndOfDay) {
		if fromTime.Weekday() == time.Sunday || fromTime.Weekday() == time.Saturday {
			year, month, day := fromTime.Date()
			beginningOfDay := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
			endOfDay := beginningOfDay.Add(24 * time.Hour)
			// adjust beginning of day
			if atpStableDesiredTime.After(beginningOfDay) {
				beginningOfDay = atpStableDesiredTime
			}
			// adjust end of day
			if toTime.Before(endOfDay) {
				endOfDay = toTime
			}

			totalWeekendDuration += endOfDay.Sub(beginningOfDay)
		}

		fromTime = fromTime.Add(24 * time.Hour)
	}

	return totalWeekendDuration
}

func getOutdatedComponent(
	stableComp s2hv1.StableComponentSpec,
	latestVersion s2hv1.DesiredImageTime,
	outdatedDuration time.Duration) s2hv1.OutdatedComponent {
	return s2hv1.OutdatedComponent{
		CurrentImage: &s2hv1.Image{
			Repository: stableComp.Repository,
			Tag:        stableComp.Version,
		},
		DesiredImage: &s2hv1.Image{
			Repository: latestVersion.Repository,
			Tag:        latestVersion.Tag,
		},
		OutdatedDuration: outdatedDuration,
	}
}
