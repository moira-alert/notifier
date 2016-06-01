package notifier

import (
	"fmt"
	"sync"
	"time"
)

// CheckSelfStateMonitorSettings - validate contact types
func CheckSelfStateMonitorSettings() error {
	if ToBool(config.Notifier.SelfState.Enabled) {
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

// SelfStateMonitor - send message when moira don't work
func SelfStateMonitor(shutdown chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	var metricsCount, checksCount int64
	checkTicker := time.NewTicker(time.Second * 10)
	lastMetricReceivedTS := GetNow().Unix()
	redisLastCheckTS := GetNow().Unix()
	lastCheckTS := GetNow().Unix()
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
			mc, _ := db.GetMetricsCount()
			cc, err := db.GetChecksCount()
			if err == nil {
				redisLastCheckTS = nowTS
				if metricsCount != mc {
					metricsCount = mc
					lastMetricReceivedTS = nowTS
				}
				if checksCount != cc {
					checksCount = cc
					lastCheckTS = nowTS
				}
			}
			if nextSendErrorMessage < nowTS {
				if redisLastCheckTS < nowTS-config.Notifier.SelfState.RedisDisconectDelay {
					log.Errorf("Redis disconnected more %ds. Send message.", nowTS-redisLastCheckTS)
					sendErrorMessages("Redis disconnected", nowTS-redisLastCheckTS, config.Notifier.SelfState.RedisDisconectDelay)
					nextSendErrorMessage = nowTS + config.Notifier.SelfState.NoticeInterval
					continue
				}
				if lastMetricReceivedTS < nowTS-config.Notifier.SelfState.LastMetricReceivedDelay && err == nil {
					log.Errorf("Moira-Cache does not received new metrics more %ds. Send message.", nowTS-lastMetricReceivedTS)
					sendErrorMessages("Moira-Cache does not received new metrics", nowTS-lastMetricReceivedTS, config.Notifier.SelfState.LastMetricReceivedDelay)
					nextSendErrorMessage = nowTS + config.Notifier.SelfState.NoticeInterval
					continue
				}
				if lastCheckTS < nowTS-config.Notifier.SelfState.LastCheckDelay && err == nil {
					log.Errorf("Moira-Checker does not checks triggers more %ds. Send message.", nowTS-lastCheckTS)
					sendErrorMessages("Moira-Checker does not checks triggers", nowTS-lastCheckTS, config.Notifier.SelfState.LastCheckDelay)
					nextSendErrorMessage = nowTS + config.Notifier.SelfState.NoticeInterval
				}
			}
		}
	}
}
func sendErrorMessages(message string, curentValue int64, errValue int64) {
	for _, adminContact := range config.Notifier.SelfState.Contacts {
		sending[adminContact["type"]] <- notificationPackage{
			Contact: ContactData{
				Type:  adminContact["type"],
				Value: adminContact["value"],
			},
			Trigger: TriggerData{
				Name:       message,
				ErrorValue: float64(errValue),
			},
			Events: []EventData{
				EventData{
					Timestamp: GetNow().Unix(),
					State:     "ERROR",
					Metric:    message,
					Value:     float64(curentValue),
				},
			},
			DontResend: true,
		}
	}
}
