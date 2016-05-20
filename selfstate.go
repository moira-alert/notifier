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
	lastMetricReceivedTS := GetNow().Unix()
	redisLastCheckTS := GetNow().Unix()
	// lastCheckTS := GetNow().Unix()
	nextSendErrorMessage := GetNow().Unix()

	log.Debugf("Start Moira Self State Monitor")
	for {
		select {
		case <-shutdown:
			checkTicker.Stop()
			log.Debugf("Stop Self State Monitor")
			return
		case <-checkTicker.C:
			nowTS := GetNow().Unix()
			var err error
			lastMetricReceivedTS, err = db.GetLastMetricReceivedTS()
			// lastCheckTS, err = db.GetLastCheckTS()
			if err == nil {
				redisLastCheckTS = nowTS
			}
			if nextSendErrorMessage < nowTS {
				if redisLastCheckTS < nowTS-config.Notifier.SelfState.RedisDisconectDelay {
					log.Errorf("Redis disconnected too long. Send messages.")
					sendErrorMessages("Redis Disconnected", nowTS)
					nextSendErrorMessage = nowTS + config.Notifier.SelfState.CkeckPeriod
					continue
				}
				if lastMetricReceivedTS < nowTS-config.Notifier.SelfState.LastMetricReceivedDelay && err == nil {
					log.Errorf("Moira-Cache does not get new metrics too long. Send messages.")
					sendErrorMessages("Moira-Cache does not get new metrics", nowTS)
					nextSendErrorMessage = nowTS + config.Notifier.SelfState.CkeckPeriod
					continue
				}
				// if lastCheckTS < nowTS-config.Notifier.SelfState.LastCheckDelay && err == nil {
				// 	log.Errorf("Moira-Checker does not checked triggers too long. Send message.")
				// 	sendErrorMessages("Moira-Checker does not checked triggers too long", lastMetricReceivedTS)
				// 	nextSendErrorMessage = nowTS + config.Notifier.SelfState.CkeckPeriod
				// }
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
					Metric:    name,
				},
			},
			DontResend: true,
		}
	}
}
