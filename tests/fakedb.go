package tests

import (
	"encoding/json"
	"fmt"
	"github.com/moira-alert/notifier"
)

var contacts = []notifier.ContactData{
	{
		ID:    "ContactID-000000000000001",
		Type:  "email",
		Value: "mail1@example.com",
	},
	{
		ID:    "ContactID-000000000000002",
		Type:  "email",
		Value: "failed@example.com",
	},
	{
		ID:    "ContactID-000000000000003",
		Type:  "email",
		Value: "mail3@example.com",
	},
	{
		ID:    "ContactID-000000000000004",
		Type:  "email",
		Value: "mail4@example.com",
	},
	{
		ID:    "ContactID-000000000000005",
		Type:  "slack",
		Value: "#devops",
	},
	{
		ID:    "ContactID-000000000000006",
		Type:  "unknown",
		Value: "no matter",
	},
}

var triggers = []notifier.TriggerData{
	{
		ID:         "triggerID-0000000000001",
		Name:       "test trigger 1",
		Targets:    []string{"test.target.1"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-1"},
	},
	{
		ID:         "triggerID-0000000000002",
		Name:       "test trigger 2",
		Targets:    []string{"test.target.2"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-2"},
	},
	{
		ID:         "triggerID-0000000000003",
		Name:       "test trigger 3",
		Targets:    []string{"test.target.3"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-3"},
	},
	{
		ID:         "triggerID-0000000000004",
		Name:       "test trigger 4",
		Targets:    []string{"test.target.4"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-4"},
	},
	{
		ID:         "triggerID-0000000000005",
		Name:       "test trigger 5 (nobody is subscribed)",
		Targets:    []string{"test.target.5"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-nosub"},
	},
	{
		ID:         "triggerID-0000000000006",
		Name:       "test trigger 6 (throttling disabled)",
		Targets:    []string{"test.target.6"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-throttling-disabled"},
	},
	{
		ID:         "triggerID-0000000000007",
		Name:       "test trigger 7 (multiple subscribers)",
		Targets:    []string{"test.target.7"},
		WarnValue:  10,
		ErrorValue: 20,
		Tags:       []string{"test-tag-multiple-subs"},
	},
}

var subscriptions = []notifier.SubscriptionData{
	{
		ID:                "subscriptionID-00000000000001",
		Enabled:           true,
		Tags:              []string{"test-tag-1"},
		Contacts:          []string{contacts[0].ID},
		ThrottlingEnabled: true,
	},
	{
		ID:       "subscriptionID-00000000000002",
		Enabled:  true,
		Tags:     []string{"test-tag-2"},
		Contacts: []string{contacts[1].ID},
		Schedule: notifier.ScheduleData{
			StartOffset:    10,
			EndOffset:      20,
			TimezoneOffset: 0,
			Days: []notifier.ScheduleDataDay{
				{Enabled: false},
				{Enabled: true}, // Tuesday 00:10 - 00:20
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
			},
		},
		ThrottlingEnabled: true,
	},
	{
		ID:       "subscriptionID-00000000000003",
		Enabled:  true,
		Tags:     []string{"test-tag-3"},
		Contacts: []string{contacts[2].ID},
		Schedule: notifier.ScheduleData{
			StartOffset:    0,   // 0:00 (GMT +5) after
			EndOffset:      900, // 15:00 (GMT +5)
			TimezoneOffset: -300,
			Days: []notifier.ScheduleDataDay{
				{Enabled: false},
				{Enabled: false},
				{Enabled: true},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
			},
		},
		ThrottlingEnabled: true,
	},
	{
		ID:       "subscriptionID-00000000000004",
		Enabled:  true,
		Tags:     []string{"test-tag-4"},
		Contacts: []string{contacts[3].ID},
		Schedule: notifier.ScheduleData{
			StartOffset:    660, // 16:00 (GMT +5) before
			EndOffset:      900, // 20:00 (GMT +5)
			TimezoneOffset: 0,
			Days: []notifier.ScheduleDataDay{
				{Enabled: false},
				{Enabled: false},
				{Enabled: true},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
			},
		},
		ThrottlingEnabled: true,
	},
	{
		ID:                "subscriptionID-00000000000005",
		Enabled:           false,
		Tags:              []string{"test-tag-1"},
		Contacts:          []string{contacts[0].ID},
		ThrottlingEnabled: true,
	},
	{
		ID:                "subscriptionID-00000000000006",
		Enabled:           false,
		Tags:              []string{"test-tag-slack"},
		Contacts:          []string{contacts[4].ID},
		ThrottlingEnabled: true,
	},
	{
		ID:                "subscriptionID-00000000000007",
		Enabled:           false,
		Tags:              []string{"unknown-contact-type"},
		Contacts:          []string{contacts[5].ID},
		ThrottlingEnabled: true,
	},
	{
		ID:                "subscriptionID-00000000000008",
		Enabled:           true,
		Tags:              []string{"test-tag-throttling-disabled"},
		Contacts:          []string{contacts[0].ID},
		ThrottlingEnabled: false,
	},
	{
		ID:       "subscriptionID-00000000000009",
		Enabled:  true,
		Tags:     []string{"test-tag-multiple-subs"},
		Contacts: []string{contacts[2].ID},
		Schedule: notifier.ScheduleData{
			StartOffset:    0,   // 0:00 (GMT +5) after
			EndOffset:      900, // 15:00 (GMT +5)
			TimezoneOffset: -300,
			Days: []notifier.ScheduleDataDay{
				{Enabled: false},
				{Enabled: false},
				{Enabled: true},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
				{Enabled: false},
			},
		},
		ThrottlingEnabled: true,
	},
	{
		ID:                "subscriptionID-00000000000010",
		Enabled:           true,
		Tags:              []string{"test-tag-multiple-subs"},
		Contacts:          []string{contacts[0].ID},
		ThrottlingEnabled: false,
	},
}

type testDatabase struct {
	conn *notifier.DbConnector
}

func (db *testDatabase) init() {
	c := db.conn.Pool.Get()
	defer c.Close()
	c.Do("FLUSHDB")
	for _, testContact := range contacts {
		testContactString, _ := json.Marshal(testContact)
		c.Do("SET", fmt.Sprintf("moira-contact:%s", testContact.ID), testContactString)
	}
	for _, testSubscription := range subscriptions {
		testSubscriptionString, _ := json.Marshal(testSubscription)
		c.Do("SET", fmt.Sprintf("moira-subscription:%s", testSubscription.ID), testSubscriptionString)
		c.Do("SADD", fmt.Sprintf("moira-tag-subscriptions:%s", testSubscription.Tags[0]), testSubscription.ID)
	}
	for _, testTrigger := range triggers {
		testTriggerString, _ := json.Marshal(testTrigger)
		c.Do("SET", fmt.Sprintf("moira-trigger:%s", testTrigger.ID), testTriggerString)
		for _, tag := range testTrigger.Tags {
			c.Do("SADD", fmt.Sprintf("moira-trigger-tags:%s", testTrigger.ID), tag)
		}
	}
}

func (db *testDatabase) getNotifications(from, to int) ([]*notifier.ScheduledNotification, error) {
	c := db.conn.Pool.Get()
	defer c.Close()

	redisResponse, err := c.Do("ZRANGE", "moira-notifier-notifications", from, to)
	if err != nil {
		return nil, err
	}
	return notifier.ConvertNotifications(redisResponse)
}

func (db *testDatabase) saveEvent(triggerID string, event *notifier.EventData) error {
	c := db.conn.Pool.Get()
	defer c.Close()

	eventString, err := json.Marshal(event)
	if err != nil {
		return err
	}

	eventsKey := fmt.Sprintf("moira-trigger-events:%s", triggerID)
	if _, err := c.Do("ZADD", eventsKey, event.Timestamp, eventString); err != nil {
		return err
	}
	return nil
}

func (db *testDatabase) getSingleNotification() (*notifier.ScheduledNotification, error) {
	notifications, err := db.getNotifications(0, -1)
	if err != nil {
		return nil, err
	}
	if len(notifications) != 1 {
		return nil, fmt.Errorf("got %d notifications instead of 1", len(notifications))
	}
	return notifications[0], nil
}
