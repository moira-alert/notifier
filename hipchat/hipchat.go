package hipchat

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/moira-alert/notifier"

	"github.com/tbruyelle/hipchat-go/hipchat"
)

var log notifier.Logger

// Sender implements moira sender interface via slack
type Sender struct {
	APIToken string
	FrontURI string
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger notifier.Logger) error {
	sender.APIToken = senderSettings["api_token"]
	if sender.APIToken == "" {
		return fmt.Errorf("Can not read hipchat api_token from config")
	}
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	api := hipchat.NewClient(sender.APIToken)

	var message bytes.Buffer
	state := events.GetSubjectState()
	tags := trigger.GetTags()
	message.WriteString(fmt.Sprintf("<b>%s</b> %s <a href=%s/#/events/%s>%s</a><br /> %s <br /><pre>", state, tags, sender.FrontURI, events[0].TriggerID, trigger.Name, trigger.Desc))
	color := hipchat.ColorGreen
	for _, event := range events {
		if event.State != "OK" {
			color = hipchat.ColorRed
		}
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message.WriteString(fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State))
		if len(event.Message) > 0 {
			message.WriteString(fmt.Sprintf(". %s", event.Message))
		}
	}

	message.WriteString("</pre>")

	if throttled {
		message.WriteString("<br />Please, <b>fix your system or tune this trigger</b> to generate less events.")
	}

	log.Debugf("Calling hipchat with message body %s", message.String())

	notification := &hipchat.NotificationRequest{
		Message:       message.String(),
		Color:         color,
		MessageFormat: "html",
	}

	_, err := api.Room.Notification(contact.Value, notification)
	if err != nil {
		return fmt.Errorf("Failed to send message to hipchat [%s]: %s", contact.Value, err.Error())
	}
	return nil
}
