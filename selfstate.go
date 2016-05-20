package notifier

import (
	"fmt"
	"sync"
	"time"
)

// CheckSelfStateMonitorSettings -
func CheckSelfStateMonitorSettings() error {
	if config.Notifier.SelfState.Enabled != "true" {
		return nil
	}
	for _, adminContact := range config.Notifier.SelfState.Contacts {
		if _, ok := sending[adminContact["type"]]; !ok {
			return fmt.Errorf("Unknown contact type [%s]", adminContact["type"])
		}
		if adminContact["value"] == "" {
			return fmt.Errorf("Value for [%s] must be present", adminContact["type"])
		}
	}
	return nil
}

// SelfStateMonitor send message if moira don't work
func SelfStateMonitor(shutdown chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	checkTicker := time.NewTicker(time.Second * 10)
	sendTicker := time.NewTicker(time.Second * time.Duration(config.Notifier.SelfState.CkeckPeriod))
	var lastMetricReceivedTS = GetNow().Unix()
	var redisLastCheckTS = GetNow().Unix()

	log.Debugf("Start Moira Self State Monitor")
	for {
		select {
		case <-shutdown:
			checkTicker.Stop()
			sendTicker.Stop()
			log.Debugf("Stop Self State Monitor")
			return
		case <-checkTicker.C:
			nowTS := GetNow().Unix()
			var err error
			lastMetricReceivedTS, err = db.GetLastMetricReceivedTS()
			if err == nil {
				redisLastCheckTS = nowTS
			}
		case <-sendTicker.C:
			nowTS := GetNow().Unix()
			if redisLastCheckTS < nowTS-config.Notifier.SelfState.RedisDisconectDelay {
				log.Errorf("Redis disconnected too long. Send messages.")
				sendErrorMessages("Redis Disconnected", redisLastCheckTS)
			}
			if lastMetricReceivedTS < nowTS-config.Notifier.SelfState.LastMetricReceivedDelay {
				log.Errorf("Moira-Cache does not get new metrics too long. Send messages.")
				sendErrorMessages("Moira-Cache does not get new metrics", lastMetricReceivedTS)
			}
		}
	}
}

func sendErrorMessages(name string, ts int64) {
	for _, adminContact := range config.Notifier.SelfState.Contacts {
		sending[adminContact["type"]] <- notificationPackage{
			Contact: ContactData{
				Type:  adminContact["type"],
				Value: adminContact["value"],
			},
			Trigger: TriggerData{
				Name: name,
			},
			Events: []EventData{
				EventData{
					Timestamp: ts,
					State:     "ERROR",
					Message:   "Message",
					Metric:    name,
				},
			},
			DontResend: true,
		}
	}
}
