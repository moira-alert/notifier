package notifier

import (
	"fmt"
	"sync"
	"time"
)

type throttlingLevel struct {
	duration time.Duration
	delay    time.Duration
	count    int64
}

// NotificationPackage respresent notifications grouped by contact type, contact value and triggerID
type notificationPackage struct {
	Events    []EventData
	Trigger   TriggerData
	Contact   ContactData
	Throttled bool
	FailCount int
}

func (pkg *notificationPackage) String() string {
	return fmt.Sprintf("package of %d notifications to %s", len(pkg.Events), pkg.Contact.Value)
}

func calculateNextDelivery(event *EventData) (time.Time, bool) {
	// if trigger switches more than .count times in .length seconds, delay next delivery for .delay seconds
	// processing stops after first condition matches
	throttlingLevels := []throttlingLevel{
		{3 * time.Hour, time.Hour, 20},
		{time.Hour, time.Hour / 2, 10},
	}

	now := GetNow()
	alarmFatigue := false

	next, beginning := db.GetTriggerThrottlingTimestamps(event.TriggerID)

	if next.After(now) {
		alarmFatigue = true
	} else {
		next = now
	}

	subscription, err := db.GetSubscription(event.SubscriptionID)
	if err != nil {
		log.Debugf("Failed get subscription by id: %s. %s", event.SubscriptionID, err.Error())
		return next, alarmFatigue
	}

	if subscription.ThrottlingEnabled {
		if next.After(now) {
			log.Debugf("Using existing throttling for trigger %s: %s", event.TriggerID, next)
		} else {
			for _, level := range throttlingLevels {
				from := now.Add(-level.duration)
				if from.Before(beginning) {
					from = beginning
				}
				count := db.GetTriggerEventsCount(event.TriggerID, from.Unix())
				if count >= level.count {
					next = now.Add(level.delay)
					log.Debugf("Trigger %s switched %d times in last %s, delaying next notification for %s", event.TriggerID, count, level.duration, level.delay)
					if err := db.SetTriggerThrottlingTimestamp(event.TriggerID, next); err != nil {
						log.Errorf("Failed to set trigger throttling timestamp: %s", err)
					}
					alarmFatigue = true
					break
				} else if count == level.count-1 {
					alarmFatigue = true
				}
			}
		}
	} else {
		next = now
	}

	next, err = subscription.Schedule.CalculateNextDelivery(next)
	if err != nil {
		log.Errorf("Failed to aply schedule for subscriptionID: %s. %s.", event.SubscriptionID, err)
	}
	return next, alarmFatigue
}

func scheduleNotification(event EventData, trigger TriggerData, contact ContactData, throttledOld bool, sendfail int) *ScheduledNotification {
	var (
		next      time.Time
		throttled bool
	)

	if sendfail > 0 {
		next = GetNow().Add(time.Minute)
		throttled = throttledOld
	} else {
		if event.State == "TEST" {
			next = GetNow()
			throttled = false
		} else {
			next, throttled = calculateNextDelivery(&event)
		}
	}

	notification := &ScheduledNotification{
		Event:     event,
		Trigger:   trigger,
		Contact:   contact,
		Throttled: throttled,
		SendFail:  sendfail,
		Timestamp: next.Unix(),
	}

	log.Debugf(
		"Scheduled notification for contact %s:%s trigger %s at %s (%d)",
		contact.Type, contact.Value, trigger.Name,
		next.Format("2006/01/02 15:04:05"), next.Unix())

	return notification
}

// FetchScheduledNotifications is a cycle that fetches scheduled notifications from database
func FetchScheduledNotifications(shutdown chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Debug("Start Fetch Sheduled Notifications")
	for {
		select {
		case <-shutdown:
			{
				log.Debug("Stop Fetch Sheduled Notifications")
				StopSenders()
				return
			}
		default:
			{
				if err := ProcessScheduledNotifications(); err != nil {
					log.Warning("Failed to fetch scheduled notifications: %s", err.Error())
				}
				time.Sleep(time.Second)
			}
		}
	}
}

// ProcessScheduledNotifications gets all notifications by now and send it
func ProcessScheduledNotifications() error {
	ts := GetNow()

	notifications, err := db.GetNotifications(ts.Unix())
	if err != nil {
		return err
	}

	notificationPackages := make(map[string]*notificationPackage)
	for _, notification := range notifications {
		packageKey := fmt.Sprintf("%s:%s:%s", notification.Contact.Type, notification.Contact.Value, notification.Event.TriggerID)
		p, found := notificationPackages[packageKey]
		if !found {
			p = &notificationPackage{
				Events:    make([]EventData, 0, len(notifications)),
				Trigger:   notification.Trigger,
				Contact:   notification.Contact,
				Throttled: notification.Throttled,
				FailCount: notification.SendFail,
			}
		}
		p.Events = append(p.Events, notification.Event)
		notificationPackages[packageKey] = p
	}

	var sendingWG sync.WaitGroup

	for _, pkg := range notificationPackages {
		ch, found := sending[pkg.Contact.Type]
		if !found {
			pkg.resend(fmt.Sprintf("Unknown contact type [%s]", pkg))
			continue
		}
		sendingWG.Add(1)
		go func(pkg *notificationPackage) {
			defer sendingWG.Done()
			log.Debugf("Start sending %s", pkg)
			select {
			case ch <- *pkg:
				break
			case <-time.After(senderTimeout):
				pkg.resend(fmt.Sprintf("Timeout sending %s", pkg))
				break
			}
		}(pkg)
	}
	sendingWG.Wait()
	return nil
}

func (pkg notificationPackage) resend(reason string) {
	sendingFailed.Mark(1)
	if metric, found := sendersFailedMetrics[pkg.Contact.Type]; found {
		metric.Mark(1)
	}
	log.Warning("Can't send message after %d try: %s. Retry again after 1 min", pkg.FailCount, reason)
	if time.Duration(pkg.FailCount)*time.Minute > resendingTimeout {
		log.Error("Stop resending. Notification interval is timed out")
	} else {
		for _, event := range pkg.Events {
			notification := scheduleNotification(event, pkg.Trigger, pkg.Contact, pkg.Throttled, pkg.FailCount+1)
			if err := db.AddNotification(notification); err != nil {
				log.Errorf("Failed to save scheduled notification: %s", err)
			}
		}
	}
}

// GetKey return notification key to prevent duplication to the same contact
func (notification *ScheduledNotification) GetKey() string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%d:%f:%d:%t:%d",
		notification.Contact.Type,
		notification.Contact.Value,
		notification.Event.TriggerID,
		notification.Event.Metric,
		notification.Event.State,
		notification.Event.Timestamp,
		notification.Event.Value,
		notification.SendFail,
		notification.Throttled,
		notification.Timestamp,
	)
}
