package twilio

import (
	"fmt"
	"strconv"
  "strings"
	"time"

	"github.com/op/go-logging"

	"github.com/moira-alert/notifier"

	twilio "github.com/carlosdp/twiliogo"
)

type SendEventsTwilio interface {
	SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error
}

type TwilioSender struct {
	client *twilio.TwilioClient
	APIFromPhone string
  log *logging.Logger
}

type TwilioSenderSms struct {
	TwilioSender
}

type TwilioSenderVoice struct {
	TwilioSender
}

func (self *TwilioSenderSms) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	var l_message string

	if len(events) == 1 {
		l_message = events[0].State + " "
	} else {
		currentValue := make(map[string]int)
		for _, event := range events {
			currentValue[event.State]++
		}
		allStates := [...]string{"OK", "WARN", "ERROR", "NODATA", "TEST"}
		for _, state := range allStates {
			if currentValue[state] > 0 {
				l_message = fmt.Sprintf("%s %s", l_message, state)
			}
		}
	}

	for _, tag := range trigger.Tags {
		l_message += "[" + tag + "]"
	}
	l_message += " " + trigger.Name + "\n\n"

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		l_message += fmt.Sprintf("%s: %s = %s (%s to %s)\n", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State)
	}

	if len(events) > 5 {
		l_message += fmt.Sprintf("\n...and %d more events.", len(events)-5)
	}

	if throttled {
		l_message += "\nPlease, fix your system or tune this trigger to generate less events."
	}

	self.log.Debug("Calling twilio sms api to phone %s and message body %s", contact.Value, l_message)
	l_twiliomessage, err := twilio.NewMessage(self.client, contact.Value, self.APIFromPhone, twilio.Body(l_message))

	if err != nil {
		return fmt.Errorf("Failed to send message to contact %s: %s", contact.Value, err.Error())
	} else {
		self.log.Debug(fmt.Sprintf("message send to twilio with status: %s", l_twiliomessage.Status))
	}

	return nil
}

func (self *TwilioSenderVoice) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	twilio.NewCall(self.client, contact.Value, self.APIFromPhone, nil)
	return nil
}

//-----------------------------------------------------------------------------
//
//
//
//-----------------------------------------------------------------------------
// Sender implements moira sender interface via twilio
type Sender struct {
	sender SendEventsTwilio
}

//Init read yaml config
func (self *Sender) Init(_senderSettings map[string]string, _logger *logging.Logger) error {
	l_APISid := _senderSettings["api_sid"]
	if l_APISid == "" {
		return fmt.Errorf("Can not read twilio api_sid from config")
	}

	l_APISecret := _senderSettings["api_secret"]
	if l_APISecret == "" {
		return fmt.Errorf("Can not read twilio api_secret from config")
	}

	l_APIFromPhone := _senderSettings["api_fromphone"]
	if l_APIFromPhone == "" {
		return fmt.Errorf("Can not read twilio from phone")
	}

	l_APItype := _senderSettings["type"][strings.Index(_senderSettings["type"], " ") + 1: len(_senderSettings["type"])]
	l_twilioClient:= twilio.NewClient(l_APISid, l_APISecret)

	switch l_APItype {
	case "sms":
		self.sender = &TwilioSenderSms{TwilioSender{l_twilioClient, l_APIFromPhone, _logger}}

	case "voice":
		self.sender = &TwilioSenderVoice{TwilioSender{l_twilioClient, l_APIFromPhone, _logger}}

	default:
		return fmt.Errorf("Wrong twilio type: %s", l_APItype)
	}

	return nil
}

//SendEvents implements Sender interface Send
func (self *Sender) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	return self.sender.SendEvents(events, contact, trigger, throttled)
}
