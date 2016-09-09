package slack

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/moira-alert/notifier"

	"github.com/nlopes/slack"
	"github.com/op/go-logging"
)

var log *logging.Logger

// Sender implements moira sender interface via slack
type Sender struct {
	APIToken string
	FrontURI string
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	sender.APIToken = senderSettings["api_token"]
	if sender.APIToken == "" {
		return fmt.Errorf("Can not read slack api_token from config")
	}
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	api := slack.New(sender.APIToken)

	var message bytes.Buffer
	state := events.GetSubjectState()
	tags := trigger.GetTags()
	message.WriteString(fmt.Sprintf("*%s* %s <%s/#/events/%s|%s>\n %s \n```", state, tags, sender.FrontURI, events[0].TriggerID, trigger.Name, trigger.Desc))
	icon := fmt.Sprintf("%s/public/fav72_ok.png", sender.FrontURI)
	for _, event := range events {
		if event.State != "OK" {
			icon = fmt.Sprintf("%s/public/fav72_error.png", sender.FrontURI)
		}
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message.WriteString(fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State))
		if len(event.Message) > 0 {
			message.WriteString(fmt.Sprintf(". %s", event.Message))
		}
	}

	message.WriteString("```")

	if throttled {
		message.WriteString("\nPlease, *fix your system or tune this trigger* to generate less events.")
	}

	log.Debugf("Calling slack with message body %s", message.String())

	params := slack.PostMessageParameters{
		Username: "Moira",
		IconURL:  icon,
	}

	_, _, err := api.PostMessage(contact.Value, message.String(), params)
	if err != nil {
		return fmt.Errorf("Failed to send message to slack [%s]: %s", contact.Value, err.Error())
	}
	return nil
}
