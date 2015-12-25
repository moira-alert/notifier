package notifier

import (
	"fmt"

	"github.com/rcrowley/go-metrics"
)

func run(sender Sender, ch chan notificationPackage) {
	defer wg.Done()
	for pkg := range ch {
		err := sender.SendEvents(pkg.Events, pkg.Contact, pkg.Trigger, pkg.Throttled)
		if err == nil {
			sendersOkMetrics[pkg.Contact.Type].Mark(1)
		} else {
			pkg.resend(err.Error())
		}
	}
}

// StopSenders close all sending channels
func StopSenders() {
	for _, ch := range sending {
		close(ch)
	}
	log.Debug("Waiting senders finish ...")
	wg.Wait()
}

// RegisterSender adds sender for notification type and registers metrics
func RegisterSender(senderSettings map[string]string, sender Sender) error {
	var senderIdent string
	if senderSettings["type"] == "script" {
		senderIdent = senderSettings["name"]
	} else {
		senderIdent = senderSettings["type"]
	}
	err := sender.Init(senderSettings, log)
	if err != nil {
		return fmt.Errorf("Don't initialize sender [%s], err [%s]", senderIdent, err.Error())
	}
	ch := make(chan notificationPackage)
	sending[senderIdent] = ch
	sendersOkMetrics[senderIdent] = metrics.NewRegisteredMeter(fmt.Sprintf("%s.sends_ok", senderIdent), metrics.DefaultRegistry)
	sendersFailedMetrics[senderIdent] = metrics.NewRegisteredMeter(fmt.Sprintf("%s.sends_failed", senderIdent), metrics.DefaultRegistry)
	wg.Add(1)
	go run(sender, ch)
	log.Debug("Sender %s registered", senderIdent)
	return nil
}
