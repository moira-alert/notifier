package tests

import (
	"fmt"
	"github.com/moira-alert/notifier"
	"time"

	"github.com/op/go-logging"
)

type badSender struct {
}

func (sender *badSender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	return nil
}

//SendEvents implements Sender interface to test notifications failure
func (sender *badSender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	if contact.Value == "failed@example.com" {
		return fmt.Errorf("I can't send notifications by design")
	}
	return nil

}

type timeoutSender struct {
}

func (sender *timeoutSender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	return nil
}

//SendEvents implements Sender interface to test notifications timeout
func (sender *timeoutSender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	time.Sleep(20 * time.Millisecond)
	return nil
}
