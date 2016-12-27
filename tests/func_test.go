// +build func

package tests

import (
	"github.com/moira-alert/notifier"
	"github.com/moira-alert/notifier/mail"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
//tcReport  = flag.Bool("teamcity", false, "enable TeamCity reporting format")
//useFakeDb = flag.Bool("fakedb", true, "use fake db instead localhost real redis")
//log       *logging.Logger
)

var _ = Describe("NotifierFunctions", func() {

	Context("Use python smtp_tls for main sender", func() {
		var (
			sender      mail.Sender
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
				Type:  "email",
				Value: "mail1@example.com",
			}
		)
		BeforeEach(func() {
			sender = mail.Sender{
				FrontURI:    "http://localhost",
				From:        "test@notifier",
				SMTPhost:    "127.0.0.1",
				SMTPport:    2500,
				InsecureTLS: true,
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
