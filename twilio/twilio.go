package twilio

import (
	"fmt"
	"strconv"
	"time"

	"github.com/op/go-logging"

	"github.com/moira-alert/notifier"

	twilio "github.com/carlosdp/twiliogo"
)

type sendEventsTwilio interface {
	SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error
}

type twilioSender struct {
	client       *twilio.TwilioClient
	APIFromPhone string
	log          *logging.Logger
}

type twilioSenderSms struct {
	twilioSender
}

type twilioSenderVoice struct {
	twilioSender
}

func (smsSender *twilioSenderSms) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
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

func (voiceSender *twilioSenderVoice) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	twilio.NewCall(voiceSender.client, voiceSender.APIFromPhone, contact.Value, nil)
	return nil
}

// Sender implements moira sender interface via twilio
type Sender struct {
	sender sendEventsTwilio
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	apiSID := senderSettings["api_sid"]
	if apiSID == "" {
		return fmt.Errorf("Can not read twilio api_sid from config")
	}

	apiSecret := senderSettings["api_secret"]
	if apiSecret == "" {
		return fmt.Errorf("Can not read twilio api_secret from config")
	}

	apiFromPhone := senderSettings["api_fromphone"]
	if apiFromPhone == "" {
		return fmt.Errorf("Can not read twilio from phone")
	}

	apiType := senderSettings["type"]
	twilioClient := twilio.NewClient(apiSID, apiSecret)

	switch apiType {
	case "twilio sms":
		sender.sender = &twilioSenderSms{twilioSender{twilioClient, apiFromPhone, logger}}

	case "twilio voice":
		sender.sender = &twilioSenderVoice{twilioSender{twilioClient, apiFromPhone, logger}}

	default:
		return fmt.Errorf("Wrong twilio type: %s", apiType)
	}

	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events []notifier.EventData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	return sender.sender.SendEvents(events, contact, trigger, throttled)
}
