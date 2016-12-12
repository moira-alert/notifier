package notifier

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

//DbConnector contains redis pool
type DbConnector struct {
	Pool *redis.Pool
}

// Database implements DB functionality
type Database interface {
	FetchEvent() (*EventData, error)
	GetTrigger(id string) (TriggerData, error)
	GetTriggerTags(id string) ([]string, error)
	GetTagsSubscriptions(tags []string) ([]SubscriptionData, error)
	GetSubscription(id string) (SubscriptionData, error)
	GetContact(id string) (ContactData, error)
	AddNotification(notification *ScheduledNotification) error
	GetTriggerThrottlingTimestamps(id string) (time.Time, time.Time)
	GetTriggerEventsCount(id string, from int64) int64
	SetTriggerThrottlingTimestamp(id string, next time.Time) error
	GetNotifications(to int64) ([]*ScheduledNotification, error)
	GetMetricsCount() (int64, error)
	GetChecksCount() (int64, error)
}

// ConvertNotifications extracts ScheduledNotification from redis response
func ConvertNotifications(redisResponse interface{}) ([]*ScheduledNotification, error) {

	notificationStrings, err := redis.Strings(redisResponse, nil)
	if err != nil {
		return nil, err
	}

	notifications := make([]*ScheduledNotification, 0, len(notificationStrings))

	for _, notificationString := range notificationStrings {
		notification := &ScheduledNotification{}
		if err := json.Unmarshal([]byte(notificationString), notification); err != nil {
			log.Warningf("Failed to parse scheduled json notification %s: %s", notificationString, err.Error())
			continue
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}

// GetNotifications fetch notifications by given timestamp
func (connector *DbConnector) GetNotifications(to int64) ([]*ScheduledNotification, error) {
	c := connector.Pool.Get()
	defer c.Close()

	c.Send("MULTI")
	c.Send("ZRANGEBYSCORE", "moira-notifier-notifications", "-inf", to)
	c.Send("ZREMRANGEBYSCORE", "moira-notifier-notifications", "-inf", to)
	redisRawResponse, err := c.Do("EXEC")
	if err != nil {
		return nil, err
	}

	redisResponse, err := redis.Values(redisRawResponse, nil)
	if err != nil {
		return nil, err
	}

	return ConvertNotifications(redisResponse[0])
}

// GetTriggerThrottlingTimestamps get throttling or scheduled notifications delay for given triggerID
func (connector *DbConnector) GetTriggerThrottlingTimestamps(triggerID string) (time.Time, time.Time) {
	c := connector.Pool.Get()
	defer c.Close()

	next, _ := redis.Int64(c.Do("GET", fmt.Sprintf("moira-notifier-next:%s", triggerID)))
	beginning, _ := redis.Int64(c.Do("GET", fmt.Sprintf("moira-notifier-throttling-beginning:%s", triggerID)))

	return time.Unix(next, 0), time.Unix(beginning, 0)
}

// SetTriggerThrottlingTimestamp store throttling or scheduled notifications delay for given triggerID
func (connector *DbConnector) SetTriggerThrottlingTimestamp(triggerID string, next time.Time) error {
	c := connector.Pool.Get()
	defer c.Close()
	if _, err := c.Do("SET", fmt.Sprintf("moira-notifier-next:%s", triggerID), next.Unix()); err != nil {
		return err
	}
	return nil
}

// GetTriggerEventsCount retuns planned notifications count from given timestamp
func (connector *DbConnector) GetTriggerEventsCount(triggerID string, from int64) int64 {
	c := connector.Pool.Get()
	defer c.Close()

	eventsKey := fmt.Sprintf("moira-trigger-events:%s", triggerID)
	count, _ := redis.Int64(c.Do("ZCOUNT", eventsKey, from, "+inf"))
	return count
}

// GetTagsSubscriptions returns all subscriptions for given tags list
func (connector *DbConnector) GetTagsSubscriptions(tags []string) ([]SubscriptionData, error) {
	c := connector.Pool.Get()
	defer c.Close()

	log.Debugf("Getting tags %v subscriptions", tags)

	tagKeys := make([]interface{}, 0, len(tags))
	for _, tag := range tags {
		tagKeys = append(tagKeys, fmt.Sprintf("moira-tag-subscriptions:%s", tag))
	}
	values, err := redis.Values(c.Do("SUNION", tagKeys...))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve subscriptions for tags %v: %s", tags, err.Error())
	}
	var subscriptions []string
	if err := redis.ScanSlice(values, &subscriptions); err != nil {
		return nil, fmt.Errorf("Failed to retrieve subscriptions for tags %v: %s", tags, err.Error())
	}
	if len(subscriptions) == 0 {
		log.Debugf("No subscriptions found for tag set %v", tags)
		return make([]SubscriptionData, 0, 0), nil
	}

	var subscriptionsData []SubscriptionData
	for _, id := range subscriptions {
		sub, err := db.GetSubscription(id)
		if err != nil {
			continue
		}
		subscriptionsData = append(subscriptionsData, sub)
	}
	return subscriptionsData, nil
}

// GetContact returns contact data by given id
func (connector *DbConnector) GetContact(id string) (ContactData, error) {
	c := connector.Pool.Get()
	defer c.Close()

	var contact ContactData

	contactString, err := redis.Bytes(c.Do("GET", fmt.Sprintf("moira-contact:%s", id)))
	if err != nil {
		return contact, fmt.Errorf("Failed to get contact data for id %s: %s", id, err.Error())
	}
	if err := json.Unmarshal(contactString, &contact); err != nil {
		return contact, fmt.Errorf("Failed to parse contact json %s: %s", contactString, err.Error())
	}
	contact.ID = id
	return contact, nil
}

// GetSubscription returns subscription data by given id
func (connector *DbConnector) GetSubscription(id string) (SubscriptionData, error) {
	c := connector.Pool.Get()
	defer c.Close()

	sub := SubscriptionData{
		ThrottlingEnabled: true,
	}
	subscriptionString, err := redis.Bytes(c.Do("GET", fmt.Sprintf("moira-subscription:%s", id)))
	if err != nil {
		subsMalformed.Mark(1)
		return sub, fmt.Errorf("Failed to get subscription data for id %s: %s", id, err.Error())
	}
	if err := json.Unmarshal(subscriptionString, &sub); err != nil {
		subsMalformed.Mark(1)
		return sub, fmt.Errorf("Failed to parse subscription json %s: %s", subscriptionString, err.Error())
	}
	sub.ID = id
	return sub, nil
}

// GetTriggerTags returns trigger tags
func (connector *DbConnector) GetTriggerTags(triggerID string) ([]string, error) {
	c := connector.Pool.Get()
	defer c.Close()

	var tags []string

	values, err := redis.Values(c.Do("SMEMBERS", fmt.Sprintf("moira-trigger-tags:%s", triggerID)))
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve tags for trigger id %s: %s", triggerID, err.Error())
	}
	if err := redis.ScanSlice(values, &tags); err != nil {
		return nil, fmt.Errorf("Failed to retrieve tags for trigger id %s: %s", triggerID, err.Error())
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("No tags found for trigger id %s", triggerID)
	}
	return tags, nil
}

// GetTrigger returns trigger data
func (connector *DbConnector) GetTrigger(id string) (TriggerData, error) {
	c := connector.Pool.Get()
	defer c.Close()

	var trigger TriggerData

	triggerString, err := redis.Bytes(c.Do("GET", fmt.Sprintf("moira-trigger:%s", id)))
	if err != nil {
		return trigger, fmt.Errorf("Failed to get trigger data for id %s: %s", id, err.Error())
	}
	if err := json.Unmarshal(triggerString, &trigger); err != nil {
		return trigger, fmt.Errorf("Failed to parse trigger json %s: %s", triggerString, err.Error())
	}

	return trigger, nil
}

// AddNotification store notification at given timestamp
func (connector *DbConnector) AddNotification(notification *ScheduledNotification) error {

	notificationString, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	c := connector.Pool.Get()
	defer c.Close()

	if _, err := c.Do("ZADD", "moira-notifier-notifications", notification.Timestamp, notificationString); err != nil {
		return err
	}

	return nil
}

// FetchEvent waiting for event from Db
func (connector *DbConnector) FetchEvent() (*EventData, error) {
	c := connector.Pool.Get()
	defer c.Close()

	var event EventData

	rawRes, err := c.Do("BRPOP", "moira-trigger-events", 1)
	if err != nil {
		log.Warningf("Failed to wait for event: %s", err.Error())
		time.Sleep(time.Second * 5)
		return nil, nil
	}
	if rawRes != nil {
		var (
			eventBytes []byte
			key        []byte
		)
		res, _ := redis.Values(rawRes, nil)
		if _, err = redis.Scan(res, &key, &eventBytes); err != nil {
			log.Warningf("Failed to parse event: %s", err.Error())
			return nil, err
		}
		if err := json.Unmarshal(eventBytes, &event); err != nil {
			log.Error(fmt.Sprintf("Failed to parse event json %s: %s", eventBytes, err.Error()))
			return nil, err
		}
		return &event, nil
	}

	return nil, nil
}

// GetMetricsCount - return metrics count received by Moira-Cache
func (connector *DbConnector) GetMetricsCount() (int64, error) {
	c := connector.Pool.Get()
	defer c.Close()
	ts, err := redis.Int64(c.Do("GET", "moira-selfstate:metrics-heartbeat"))
	if err == redis.ErrNil {
		return 0, nil
	}
	return ts, err
}

// GetChecksCount - return checks count by Moira-Checker
func (connector *DbConnector) GetChecksCount() (int64, error) {
	c := connector.Pool.Get()
	defer c.Close()
	ts, err := redis.Int64(c.Do("GET", "moira-selfstate:checks-counter"))
	if err == redis.ErrNil {
		return 0, nil
	}
	return ts, err
}

// GetIDByUsername read ID of user by messenger username
func (connector *DbConnector) GetIDByUsername(messenger, username string) (string, error) {
	if strings.HasPrefix(username, "#") {
		result := "@" + username[1:]
		log.Debugf("Channel %s requested. Returning id: %s", username, result)
		return result, nil
	}

	c := connector.Pool.Get()
	defer c.Close()

	result, err := redis.String(c.Do("GET", fmt.Sprintf("moira-%s-users:%s", messenger, username)))

	return result, err
}

// SetUsernameID store id of username
func (connector *DbConnector) SetUsernameID(messenger, username, id string) error {
	c := connector.Pool.Get()
	defer c.Close()
	if _, err := c.Do("SET", usernameKey(messenger, username), id); err != nil {
		return err
	}
	return nil
}

func usernameKey(messenger, username string) string {
	return fmt.Sprintf("moira-%s-users:%s", messenger, username)
}

const (
	botUsername  = "moira-bot-host"
	deregistered = "deregistered"
)

var messengers = make(map[string]bool)

// RegisterBotIfAlreadyNot creates registration of bot instance in redis
func (connector *DbConnector) RegisterBotIfAlreadyNot(messenger string) bool {
	host, _ := os.Hostname()
	redisKey := usernameKey(messenger, botUsername)
	c := connector.Pool.Get()
	defer c.Close()

	c.Send("WATCH", redisKey)

	status, err := redis.Bytes(c.Do("GET", redisKey))
	statusStr := string(status)
	if err != nil {
		log.Info(err)
	}
	if statusStr == "" || statusStr == host || statusStr == deregistered {
		c.Send("MULTI")
		c.Send("SET", redisKey, host)
		_, err := c.Do("EXEC")
		if err != nil {
			log.Info(err)
			return false
		}
		messengers[messenger] = true
		return true
	}

	return false
}

// DeregisterBots cancels registration for all registered messengers
func (connector *DbConnector) DeregisterBots() {
	for messenger, flag := range messengers {
		if flag {
			connector.DeregisterBot(messenger)
		}
	}
}

// DeregisterBot removes registration of bot instance in redis
func (connector *DbConnector) DeregisterBot(messenger string) error {
	status, _ := connector.GetIDByUsername(messenger, botUsername)
	host, _ := os.Hostname()
	if status == host {
		log.Debugf("Bot for %s on host %s exists. Removing registration.", messenger, host)
		return connector.SetUsernameID(messenger, botUsername, deregistered)
	}

	log.Debugf("Notifier on host %s did't exist. Removing skipped.", host)
	return nil
}

// InitRedisDatabase creates Redis pool based on config
func InitRedisDatabase(config RedisConfig) *DbConnector {
	db := DbConnector{
		Pool: NewRedisPool(fmt.Sprintf("%s:%s", config.Host, config.Port), config.DBID),
	}
	return &db
}

// NewRedisPool creates Redis pool
func NewRedisPool(redisURI string, dbID ...int) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisURI)
			if err != nil {
				return nil, err
			}
			if len(dbID) > 0 {
				c.Do("SELECT", dbID[0])
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
