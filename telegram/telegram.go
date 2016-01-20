package telegram

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/moira-alert/notifier"

	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/op/go-logging"
)

var log *logging.Logger
var telegramMessageLimit = 4096

// Sender implements moira sender interface via telegram
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
	bot, err := tgbotapi.NewBotAPI(sender.APIToken)
	if err != nil {
		return fmt.Errorf("Failed to init telegram api: %s", err.Error())
	}

	var message bytes.Buffer

	state := events.GetSubjectState()
	tags := trigger.GetTags()

	message.WriteString(fmt.Sprintf("%s %s %s (%d)\n\n", state, trigger.Name, tags, len(events)))

	messageLimitReached := false
	lineCount := 0

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		line := fmt.Sprintf("%s: %s = %s (%s to %s)\n", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State)
		if message.Len()+len(line) > telegramMessageLimit-200 {
			messageLimitReached = true
			break
		}
		message.WriteString(line)
		lineCount++
	}

	if messageLimitReached {
		message.WriteString(fmt.Sprintf("\n...and %d more events.", len(events)-lineCount))
	}

	if throttled {
		message.WriteString("\nPlease, fix your system or tune this trigger to generate less events.")
	}

	log.Debug("Calling telegram api with chat_id %s and message body %s", contact.Value, message)

	telegramParams := url.Values{}
	telegramParams.Set("chat_id", contact.Value)
	telegramParams.Set("text", message.String())
	telegramParams.Set("disable_web_page_preview", "true")

	_, err = bot.MakeRequest("sendMessage", telegramParams)
	if err != nil {
		return fmt.Errorf("Failed to send message to telegram contact %s: %s", contact.Value, err.Error())
	}
	return nil

}
