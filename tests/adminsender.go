package tests

import (
	"github.com/moira-alert/notifier"

	"github.com/op/go-logging"
)


type adminSender struct {
	lastEvents notifier.EventsData
}

func (sender *adminSender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	return nil
}

func (sender *adminSender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	sender.lastEvents = events
	return nil
}
