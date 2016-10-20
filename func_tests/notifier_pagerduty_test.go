package tests

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/moira-alert/notifier"
	"github.com/moira-alert/notifier/pagerduty"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/op/go-logging"
)

var (
	tcReport  = flag.Bool("teamcity", false, "enable TeamCity reporting format")
	useFakeDb = flag.Bool("fakedb", true, "use fake db instead localhost real redis")
	log       *logging.Logger
)

func TestNotifier(t *testing.T) {
	flag.Parse()

	RegisterFailHandler(Fail)
	if *tcReport {
		RunSpecsWithCustomReporters(t, "Notifier Suite", []Reporter{reporters.NewTeamCityReporter(os.Stdout)})
	} else {
		RunSpecs(t, "Notifier Suite")
	}
}

var _ = Describe("Notifier", func() {

	BeforeSuite(func() {
		log, _ = logging.GetLogger("notifier")
		notifier.SetLogger(log)
		logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}\t%{level}\t%{message}"))
		logBackend := logging.NewLogBackend(os.Stdout, "", 0)
		logBackend.Color = false
		logging.SetBackend(logBackend)
		logging.SetLevel(logging.DEBUG, "notifier")
	})

	Context("Send alert via pagerduty", func() {
		var (
			sender      pagerduty.Sender
			err         error
			triggerData = notifier.TriggerData{
				ID:         "triggerID-0000000000001",
				Name:       "test trigger 1",
				Targets:    []string{"test.target.1"},
				WarnValue:  10,
				ErrorValue: 20,
				Tags:       []string{"test-tag-1"},
			}
			contactData = notifier.ContactData{
				ID:    "ContactID-000000000000001",
				Type:  "pagerduty",
				Value: "alxschwrz@gmail.com",
			}
		)
		BeforeEach(func() {
			sender = pagerduty.Sender{
				FrontURI: "http://localhost",
				APIToken: "a3d988470c2b4c63940cdd8887032140",
			}
			sender.SetLogger(log)
			events := make([]notifier.EventData, 0, 10)
			for event := range generateTestEvents(10, triggerData.ID) {
				events = append(events, *event)
			}
			err = sender.SendEvents(events, contactData, triggerData, true)
		})

		It("Should succeed", func() {
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

func generateTestEvents(n int, subscriptionID string) chan *notifier.EventData {
	ch := make(chan *notifier.EventData)
	go func() {
		for i := 0; i < n; i++ {
			event := &notifier.EventData{
				Metric:         fmt.Sprintf("Metric number #%d", i),
				SubscriptionID: subscriptionID,
				State:          "TEST",
			}

			ch <- event
		}
		close(ch)
	}()
	return ch
}
