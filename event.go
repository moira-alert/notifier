package notifier

import (
	"sync"
)

var eventStates = [...]string{"OK", "WARN", "ERROR", "NODATA", "TEST"}

// ProcessEvent generate notifications from EventData
func ProcessEvent(event EventData) error {
	var (
		subscriptions []SubscriptionData
		tags          []string
		trigger       TriggerData
		err           error
	)

	if event.State != "TEST" {
		log.Debug("Processing trigger id %s for metric %s == %f, %s -> %s", event.TriggerID, event.Metric, event.Value, event.OldState, event.State)

		trigger, err = db.GetTrigger(event.TriggerID)
		if err != nil {
			return err
		}

		tags, err = db.GetTriggerTags(event.TriggerID)
		if err != nil {
			return err
		}
		trigger.Tags = tags
		tags = append(tags, event.State, event.OldState)

		log.Debug("Getting subscriptions for tags %v", tags)
		subscriptions, err = db.GetTagsSubscriptions(tags)
		if err != nil {
			return err
		}
	} else {
		log.Debug("Getting subscription id %s for test message", event.SubscriptionID)
		sub, err := db.GetSubscription(event.SubscriptionID)
		if err != nil {
			return err
		}
		subscriptions = []SubscriptionData{sub}
	}

	for _, subscription := range subscriptions {
		if event.State == "TEST" || (subscription.Enabled && subset(subscription.Tags, tags)) {
			log.Debug("Processing contact ids %v for subscription %s", subscription.Contacts, subscription.ID)
			for _, contactID := range subscription.Contacts {
				contact, err := db.GetContact(contactID)
				if err != nil {
					log.Warning(err.Error())
					continue
				}
				event.SubscriptionID = subscription.ID
				if err := scheduleNotification(event, trigger, contact, false, 0); err != nil {
					log.Warning("Failed to schedule notification: %s", err.Error())
				}
			}
		} else if !subscription.Enabled {
			log.Debug("Subscription %s is disabled", subscription.ID)
		} else {
			log.Debug("Subscription %s has extra tags", subscription.ID)
		}
	}
	return nil
}

// FetchEvents is a cycle that fetches events from database
func FetchEvents(shutdown chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Debug("Start Fetching Events")
	for {
		select {
		case <-shutdown:
			{
				log.Debug("Stop Fetching Events")
				return
			}
		default:
			{
				event, err := db.FetchEvent()
				if err != nil {
					eventsMalformed.Mark(1)
					continue
				}
				if event != nil {
					eventsReceived.Mark(1)
					if err := ProcessEvent(*event); err != nil {
						eventsProcessingFailed.Mark(1)
						log.Errorf("Failed processEvent. %s", err)
					}
				}
			}
		}
	}
}

// GetSubjectState returns the most critial state of events
func (events EventsData) GetSubjectState() string{
	result := ""
	states := make(map[string]bool)
	for _, event := range events {
		states[event.State] = true
	}
	for _, state := range eventStates {
		if states[state] {
			result = state
		}
	}
	return result
}