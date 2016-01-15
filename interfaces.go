package notifier

import "github.com/op/go-logging"

// EventData represents trigger state changes event
type EventData struct {
	Timestamp      int64   `json:"timestamp"`
	Metric         string  `json:"metric"`
	Value          float64 `json:"value"`
	State          string  `json:"state"`
	TriggerID      string  `json:"trigger_id"`
	SubscriptionID string  `json:"sub_id"`
	OldState       string  `json:"old_state"`
}

// EventsData represents slice of EventData
type EventsData []EventData

// TriggerData represents trigger object
type TriggerData struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Targets    []string `json:"targets"`
	Patterns   []string `json:"patterns"`
	WarnValue  float64  `json:"warn_value"`
	ErrorValue float64  `json:"error_value"`
	Tags       []string `json:"__notifier_trigger_tags"`
}

// ContactData represents contact object
type ContactData struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	ID    string `json:"id"`
}

//SubscriptionData respresent user subscription
type SubscriptionData struct {
	Contacts          []string     `json:"contacts"`
	Enabled           bool         `json:"enabled"`
	Tags              []string     `json:"tags"`
	Schedule          ScheduleData `json:"sched"`
	ID                string       `json:"id"`
	ThrottlingEnabled bool         `json:"throttling"`
}

// ScheduleData respresent subscription schedule
type ScheduleData struct {
	Days           []ScheduleDataDay `json:"days"`
	TimezoneOffset int64             `json:"tzOffset"`
	StartOffset    int64             `json:"startOffset"`
	EndOffset      int64             `json:"endOffset"`
}

// ScheduleDataDay respresent week day of schedule
type ScheduleDataDay struct {
	Enabled bool `json:"enabled"`
}

// ScheduledNotification respresent notification object
type ScheduledNotification struct {
	Event     EventData   `json:"event"`
	Trigger   TriggerData `json:"trigger"`
	Contact   ContactData `json:"contact"`
	Throttled bool        `json:"throttled"`
	SendFail  int         `json:"send_fail"`
	Timestamp int64       `json:"timestamp"`
}

// Sender interface for implementing specified contact type sender
type Sender interface {
	SendEvents(events EventsData, contact ContactData, trigger TriggerData, throttled bool) error
	Init(senderSettings map[string]string, logger *logging.Logger) error
}
