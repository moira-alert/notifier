package pagerduty

import (
	"fmt"
	"github.com/PagerDuty/go-pagerduty"
	"io/ioutil"
	"log"
	"net/http"
)

var log *logging.Logger

// Sender implements moira sender interface via pagerduty
type Sender struct {
	APIToken string
	FrontURI string
}

// Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, logger *logging.Logger) error {
	sender.APIToken = senderSettings["api_token"]
	if sender.APIToken == "" {
		return fmt.Errorf("Can not read pagerduty api_token from config")
	}
	log = logger
	sender.FrontURI = senderSettings["front_uri"]
	return nil
}

// SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events notifier.EventsData, contact notifier.ContactData, trigger notifier.TriggerData, throttled bool) error {

	// Body JSON:
	// service_key (required): string
	//    The GUID of one of your "Generic API" services. This is the "Integration Key" listed on a Generic API's service detail page.
	//
	// event_type (required): string
	//    The type of event. Can be trigger, acknowledge or resolve.
	//
	// incident_key (required): string
	//    Identifies the incident to trigger, acknowledge, or resolve. Required unless the event_type is trigger.
	//
	// description (required): string
	//    Text that will appear in the incident's log associated with this event. Required for trigger events.
	//
	// details: object
	//    An arbitrary JSON object containing any data you'd like included in the incident log.
	//
	// client: string
	//    The name of the monitoring client that is triggering this event. (This field is only used for trigger events.)
	//
	// client_url: string
	//    The URL of the monitoring client that is triggering this event. (This field is only used for trigger events.)
	//
	// contexts: array of objects
	//    Contexts to be included with the incident trigger such as links to graphs or images. (This field is only used for trigger events.)

	subjectState := events.GetSubjectState()
	title := fmt.Sprintf("%s %s %s (%d)", subjectState, trigger.Name, trigger.GetTags(), len(events))
	timestamp := events[len(events)-1].Timestamp

	var message bytes.Buffer

	for _, event := range events {
		value := strconv.FormatFloat(event.Value, 'f', -1, 64)
		message.WriteString(fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).Format("15:04"), event.Metric, value, event.OldState, event.State))
		if len(event.Message) > 0 {
			message.WriteString(fmt.Sprintf(". %s", event.Message))
		}
	}

	if throttled {
		message.WriteString("\nPlease, fix your system or tune this trigger to generate less events.")
	}

	log.Debugf("Calling pagerduty with message title %s, body %s", title, message.String())

	event := pagerduty.Event{
		Type:        "trigger",
		ServiceKey:  sender.APIToken,
		Description: title,
		Client:      "Moira",
		ClientURL:   fmt.Sprintf("%s/#/events/%s", sender.FrontURI, events[0].TriggerID),
		Details:     message.String(),
		Contexts: ""
	}
	resp, err := pagerduty.CreateEvent(event)
	if err != nil {
		log.Println(resp)
		log.Fatalln("ERROR:", err)
	}
	log.Println("Incident key:", resp.IncidentKey)
}
