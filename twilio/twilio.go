package twilio

import (
	"fmt"
	"strconv"
	"time"
	"bytes"

	"github.com/op/go-logging"

	"github.com/moira-alert/notifier"

	twilio "github.com/carlosdp/twiliogo"
)

type sendEventsTwilio interface {
	SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error
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
	voiceURL string
}

func (smsSender *twilioSenderSms) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	var message bytes.Buffer

	state := events.GetSubjectState()
	tags := trigger.GetTags()

	message.WriteString(fmt.Sprintf("%s %s %s (%d)\n\n", state, trigger.Name, tags, len(events)))

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message.WriteString(fmt.Sprintf("%s: %s = %s (%s to %s)\n", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State))
	}

	if len(events) > 5 {
		message.WriteString(fmt.Sprintf("\n...and %d more events.", len(events)-5))
	}

	if throttled {
		message.WriteString("\nPlease, fix your system or tune this trigger to generate less events.")
	}

	smsSender.log.Debugf("Calling twilio sms api to phone %s and message body %s", contact.Value, message.String())
	twilioMessage, err := twilio.NewMessage(smsSender.client, smsSender.APIFromPhone, contact.Value, twilio.Body(message.String()))

	if err != nil {
		return fmt.Errorf("Failed to send message to contact %s: %s", contact.Value, err)
	}

	smsSender.log.Debugf(fmt.Sprintf("message send to twilio with status: %s", twilioMessage.Status))

	return nil
}

func (voiceSender *twilioSenderVoice) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	twilioCall, err := twilio.NewCall(voiceSender.client, voiceSender.APIFromPhone, contact.Value, twilio.Callback(voiceSender.voiceURL))

	if err != nil {
		return fmt.Errorf("Failed to make call to contact %s: %s", contact.Value, err)
	}

	voiceSender.log.Debugf("Call queued to twilio with status: %s", twilioCall.Status)

	return nil
}

// Sender implements moira sender interface via twilio
type Sender struct {
	sender sendEventsTwilio
}

//Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	apiType := senderSettings["type"]

	apiASID := senderSettings["api_asid"]
	if apiASID == "" {
		return fmt.Errorf("Can not read [%s] api_sid param from config", apiType)
	}

	apiAuthToken := senderSettings["api_authtoken"]
	if apiAuthToken == "" {
		return fmt.Errorf("Can not read [%s] api_authtoken param from config", apiType)
	}

	apiFromPhone := senderSettings["api_fromphone"]
	if apiFromPhone == "" {
		return fmt.Errorf("Can not read [%s] api_fromphone param from config", apiType)
	}

	twilioClient := twilio.NewClient(apiASID, apiAuthToken)

	switch apiType {
	case "twilio sms":
		sender.sender = &twilioSenderSms{twilioSender{twilioClient, apiFromPhone, logger}}

	case "twilio voice":
		voiceURL := senderSettings["voiceurl"]
		if voiceURL == "" {
			return fmt.Errorf("Can not read [%s] voiceurl param from config", apiType)
		}

		sender.sender = &twilioSenderVoice{twilioSender{twilioClient, apiFromPhone, logger}, voiceURL}

	default:
		return fmt.Errorf("Wrong twilio type: %s", apiType)
	}

	return nil
}

//SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {
	return sender.sender.SendEvents(events, contact, trigger, throttled)
}
