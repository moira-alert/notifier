package notifier

import (
	"fmt"
	"time"
)

// CalculateNextDelivery return first avaiable timestamp according to schedule
func (schedule *ScheduleData) CalculateNextDelivery(nextTime time.Time) (time.Time, error) {

	if len(schedule.Days) != 0 && len(schedule.Days) != 7 {
		return nextTime, fmt.Errorf("Invalid scheduled settings: %d days defined", len(schedule.Days))
	}

	if len(schedule.Days) == 0 {
		return nextTime, nil
	}

	tzOffset := time.Duration(schedule.TimezoneOffset) * time.Minute
	localNextTime := nextTime.Add(-tzOffset).Truncate(time.Minute)
	beginOffset := time.Duration(schedule.StartOffset) * time.Minute
	endOffset := time.Duration(schedule.EndOffset) * time.Minute
	localNextTimeDay := localNextTime.Truncate(24 * time.Hour)
	localNextWeekday := int(localNextTimeDay.Weekday()+6) % 7

	if schedule.Days[localNextWeekday].Enabled &&
		(localNextTime.Equal(localNextTimeDay.Add(beginOffset)) || localNextTime.After(localNextTimeDay.Add(beginOffset))) &&
		(localNextTime.Equal(localNextTimeDay.Add(endOffset)) || localNextTime.Before(localNextTimeDay.Add(endOffset))) {
		return nextTime, nil
	}

	// find first allowed day
	for i := 0; i < 8; i++ {
		nextLocalDayBegin := localNextTimeDay.Add(time.Duration(i*24) * time.Hour)
		nextLocalWeekDay := int(nextLocalDayBegin.Weekday()+6) % 7
		if localNextTime.After(nextLocalDayBegin.Add(beginOffset)) {
			continue
		}
		if !schedule.Days[nextLocalWeekDay].Enabled {
			continue
		}
		return nextLocalDayBegin.Add(beginOffset + tzOffset), nil
	}

	return nextTime, fmt.Errorf("Can not find allowed schedule day")
}
