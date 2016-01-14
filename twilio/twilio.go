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

type _SendEventsTwilio interface {
	SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error
}

type _TwilioSender struct {
	client       *twilio.TwilioClient
	APIFromPhone string
	log          *logging.Logger
}

type _TwilioSenderSms struct {
	_TwilioSender
}

type _TwilioSenderVoice struct {
	_TwilioSender
}

func (smsSender *_TwilioSenderSms) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	var lmessage string

	if len(events) == 1 {
		lmessage = events[0].State + " "
	} else {
		currentValue := make(map[string]int)
		for _, event := range events {
			currentValue[event.State]++
		}
		allStates := [...]string{"OK", "WARN", "ERROR", "NODATA", "TEST"}
		for _, state := range allStates {
			if currentValue[state] > 0 {
				lmessage = fmt.Sprintf("%s %s", lmessage, state)
			}
		}
	}

	for _, tag := range trigger.Tags {
		lmessage += "[" + tag + "]"
	}
	lmessage += " " + trigger.Name + "\n\n"

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		lmessage += fmt.Sprintf("%s: %s = %s (%s to %s)\n", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State)
	}

	if len(events) > 5 {
		lmessage += fmt.Sprintf("\n...and %d more events.", len(events)-5)
	}

	if throttled {
		lmessage += "\nPlease, fix your system or tune this trigger to generate less events."
	}

	smsSender.log.Debug("Calling twilio sms api to phone %s and message body %s", contact.Value, lmessage)
	ltwiliomessage, err := twilio.NewMessage(smsSender.client, smsSender.APIFromPhone, contact.Value, twilio.Body(lmessage))

	if err != nil {
		return fmt.Errorf("Failed to send message to contact %s: %s", contact.Value, err.Error())
	}

	smsSender.log.Debug(fmt.Sprintf("message send to twilio with status: %s", ltwiliomessage.Status))

	return nil
}

func (voiceSender *_TwilioSenderVoice) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	twilio.NewCall(voiceSender.client, voiceSender.APIFromPhone, contact.Value, nil)
	return nil
}

//-----------------------------------------------------------------------------
//
//
//
//-----------------------------------------------------------------------------

// Sender implements moira sender interface via twilio
type Sender struct {
	sender _SendEventsTwilio
}

//Init read yaml config
func (sender *Sender) Init(_senderSettings map[string]string, _logger *logging.Logger) error {
	lAPISid := _senderSettings["api_sid"]
	if lAPISid == "" {
		return fmt.Errorf("Can not read twilio api_sid from config")
	}

	lAPISecret := _senderSettings["api_secret"]
	if lAPISecret == "" {
		return fmt.Errorf("Can not read twilio api_secret from config")
	}

	lAPIFromPhone := _senderSettings["api_fromphone"]
	if lAPIFromPhone == "" {
		return fmt.Errorf("Can not read twilio from phone")
	}

	lAPItype := _senderSettings["type"][strings.Index(_senderSettings["type"], " ")+1 : len(_senderSettings["type"])]
	ltwilioClient := twilio.NewClient(lAPISid, lAPISecret)

	switch lAPItype {
	case "sms":
		sender.sender = &_TwilioSenderSms{_TwilioSender{ltwilioClient, lAPIFromPhone, _logger}}

	case "voice":
		sender.sender = &_TwilioSenderVoice{_TwilioSender{ltwilioClient, lAPIFromPhone, _logger}}

	default:
		return fmt.Errorf("Wrong twilio type: %s", lAPItype)
	}

	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	return sender.sender.SendEvents(events, contact, trigger, throttled)
}
