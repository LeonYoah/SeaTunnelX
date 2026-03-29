package schedulex

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const DefaultTimezone = "Asia/Shanghai"

var Parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func NormalizeTimezone(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultTimezone
	}
	return trimmed
}

func LoadLocation(value string) (*time.Location, error) {
	return time.LoadLocation(NormalizeTimezone(value))
}

func Validate(expr string) error {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return fmt.Errorf("cron expression is empty")
	}
	_, err := Parser.Parse(trimmed)
	return err
}

func MatchMinuteWindow(expr string, now time.Time, locationName string) (bool, time.Time, time.Time, error) {
	schedule, err := Parser.Parse(strings.TrimSpace(expr))
	if err != nil {
		return false, time.Time{}, time.Time{}, err
	}
	loc, err := LoadLocation(locationName)
	if err != nil {
		return false, time.Time{}, time.Time{}, err
	}
	localized := now.In(loc).Truncate(time.Minute)
	previousMinute := localized.Add(-time.Minute)
	if !schedule.Next(previousMinute).Equal(localized) {
		return false, time.Time{}, time.Time{}, nil
	}
	return true, localized, localized.Add(time.Minute), nil
}

func NextRun(expr string, from time.Time, locationName string) (*time.Time, error) {
	schedule, err := Parser.Parse(strings.TrimSpace(expr))
	if err != nil {
		return nil, err
	}
	loc, err := LoadLocation(locationName)
	if err != nil {
		return nil, err
	}
	next := schedule.Next(from.In(loc))
	return &next, nil
}
