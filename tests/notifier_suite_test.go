package tests

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"gopkg.in/gomail.v2"

	"github.com/moira-alert/notifier"
	"github.com/moira-alert/notifier/mail"

	"github.com/garyburd/redigo/redis"
	"github.com/gmlexx/redigomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/op/go-logging"
)

type testSettings struct {
	dict map[string]map[string]string
}

func (c testSettings) Get(section string, key string) string {
	if section, found := c.dict[section]; found {
		if value, found := section[key]; found {
			return value
		}
	}
	return ""
}

func (c testSettings) GetInterface(section, key string) interface{} {
	return nil
}

var (
	tcReport   = flag.Bool("teamcity", false, "enable TeamCity reporting format")
	useFakeDb  = flag.Bool("fakedb", true, "use fake db instead localhost real redis")
	log        *logging.Logger
	testDb     *testDatabase
	testConfig = testSettings{
		make(map[string]map[string]string),
	}
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
	var err error
	var event notifier.EventData

	BeforeSuite(func() {
		log, _ = logging.GetLogger("notifier")
		notifier.SetLogger(log)
		senderSettings := make(map[string]string)
		senderSettings["type"] = "email"
		notifier.RegisterSender(senderSettings, &badSender{})
		senderSettings["type"] = "slack"
		notifier.RegisterSender(senderSettings, &timeoutSender{})
		testConfig.dict["notifier"] = make(map[string]string)
		testConfig.dict["notifier"]["sender_timeout"] = "0s10ms"
		testConfig.dict["notifier"]["resending_timeout"] = "24:00"
		notifier.SetSettings(testConfig)
		logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}\t%{level}\t%{message}"))
		logBackend := logging.NewLogBackend(os.Stdout, "", 0)
		logBackend.Color = false
		logging.SetBackend(logBackend)
		logging.SetLevel(logging.DEBUG, "notifier")
		now := notifier.GetNow()
		log.Debug("Using now time: %s, %s", now, now.Weekday())
	})

	AfterSuite(func() {
		notifier.StopSenders()
	})

	BeforeEach(func() {
		testDb = &testDatabase{&notifier.DbConnector{}}
		if *useFakeDb {
			c := redigomock.NewFakeRedis()
			testDb.conn.Pool = &redis.Pool{
				MaxIdle:     3,
				IdleTimeout: 240 * time.Second,
				Dial: func() (redis.Conn, error) {
					return c, nil
				},
			}
		} else {
			testDb.conn.Pool = notifier.NewRedisPool("localhost:6379")
		}
		testDb.init()
		notifier.SetDb(testDb.conn)

		notifier.GetNow = func() time.Time {
			return time.Unix(1441188915, 0) // 2 Сентябрь 2015 г. 15:15:15 (GMT +5)
		}
	})

	Context("When one invalid event arrives", func() {
		BeforeEach(func() {
			event = notifier.EventData{
				Metric:    "test1",
				State:     "OK",
				OldState:  "WARN",
				TriggerID: "",
			}
			err = notifier.ProcessEvent(event)
		})

		It("should cause an error in processing", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When one valid event arrives", func() {
		Context("When event is TEST and subscription is disabled", func() {
			BeforeEach(func() {
				event = notifier.EventData{
					State:          "TEST",
					SubscriptionID: subscriptions[4].ID,
				}
				err = notifier.ProcessEvent(event)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should send notification now", func() {
				assertNotificationSent(event, notifier.GetNow(), contacts[0])
			})
		})

		Context("When events is TEST and one of them has unknown contact type", func() {
			BeforeEach(func() {
				assertProcessEvent(notifier.EventData{
					State:          "TEST",
					SubscriptionID: subscriptions[6].ID,
				}, false)
				assertProcessEvent(notifier.EventData{
					State:          "TEST",
					SubscriptionID: subscriptions[4].ID,
				}, false)
				err = notifier.ProcessScheduledNotifications()
			})
			It("should not processed without error", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("one notification should be sent, one rescheduled", func() {
				notifications, err := testDb.getNotifications(0, -1)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(notifications)).To(Equal(1))
				Expect(notifications[0].SendFail).To(Equal(1))
				Expect(notifications[0].Contact.ID).To(Equal(contacts[5].ID))
				Expect(notifications[0].Timestamp).To(Equal(notifier.GetNow().Add(time.Minute).Unix()))
			})
		})

		Context("When event is TEST and current interval is not allowed", func() {
			BeforeEach(func() {
				event = notifier.EventData{
					State:          "TEST",
					SubscriptionID: subscriptions[1].ID,
				}
				err = notifier.ProcessEvent(event)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should send notification now", func() {
				assertNotificationSent(event, notifier.GetNow(), contacts[1])
			})

			Context("When sending notification failure", func() {
				BeforeEach(func() {
					err = notifier.ProcessScheduledNotifications()
					Expect(err).ShouldNot(HaveOccurred())
				})
				It("notification should be rescheduled after 1 min", func() {
					notifications, err := testDb.getNotifications(0, -1)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(notifications)).To(Equal(1))
					Expect(notifications[0].SendFail).To(Equal(1))
					Expect(notifications[0].Timestamp).To(Equal(notifier.GetNow().Add(time.Minute).Unix()))
				})
			})
		})

		Context("When sending notification timeout", func() {
			BeforeEach(func() {
				for event := range generateTestEvents(2, subscriptions[5].ID) {
					err = notifier.ProcessEvent(*event)
					Expect(err).ShouldNot(HaveOccurred())
					err = notifier.ProcessScheduledNotifications()
					Expect(err).ShouldNot(HaveOccurred())
				}
			})

			It("second notification should be rescheduled after 1 min", func() {
				notifications, err := testDb.getNotifications(0, -1)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(notifications)).To(Equal(1))
				Expect(notifications[0].SendFail).To(Equal(1))
				Expect(notifications[0].Timestamp).To(Equal(notifier.GetNow().Add(time.Minute).Unix()))
			})
		})

		Context("When nobody is subscribed", func() {
			BeforeEach(func() {
				event = notifier.EventData{
					Metric:    "generate.event.1",
					State:     "OK",
					OldState:  "WARN",
					TriggerID: triggers[4].ID,
				}
				err = notifier.ProcessEvent(event)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should not send notifications", func() {
				notifications, err := testDb.getNotifications(0, -1)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(notifications).To(BeEmpty())
			})
		})

		Context("When somebody is subscribed", func() {
			Context("When he has no schedule", func() {
				BeforeEach(func() {
					event = notifier.EventData{
						Metric:    "generate.event.1",
						State:     "OK",
						OldState:  "WARN",
						TriggerID: triggers[0].ID,
					}
				})

				Context("When trigger is not throttled", func() {
					BeforeEach(func() {
						err = notifier.ProcessEvent(event)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("should send notification now", func() {
						assertNotificationSent(event, notifier.GetNow(), contacts[0])
					})
				})

				Context("When trigger is throttled", func() {
					BeforeEach(func() {
						testDb.conn.SetTriggerThrottlingTimestamp(triggers[0].ID, notifier.GetNow().Add(time.Hour))
						err = notifier.ProcessEvent(event)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("should send notification on planned time", func() {
						assertNotificationSent(event, notifier.GetNow().Add(time.Hour), contacts[0])
					})
				})
			})

			Context("When he has schedule", func() {
				BeforeEach(func() {
					event = notifier.EventData{
						Metric:    "generate.event.1",
						State:     "OK",
						OldState:  "WARN",
						TriggerID: triggers[2].ID,
					}
				})

				Context("When current time is allowed", func() {
					BeforeEach(func() {
						notifier.GetNow = func() time.Time {
							return time.Unix(1441187115, 0) // 2 Сентябрь 2015 г. 14:45:15 (GMT +5)
						}
						err = notifier.ProcessEvent(event)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("should send notification now", func() {
						assertNotificationSent(event, notifier.GetNow(), contacts[2])
					})
				})

				Context("When allowed time is today", func() {
					BeforeEach(func() {
						event.TriggerID = triggers[3].ID
						err = notifier.ProcessEvent(event)
						Expect(err).ShouldNot(HaveOccurred())
					})

					It("should send notification at the beginning of allowed interval", func() {
						assertNotificationSent(event, time.Unix(1441191600, 0), contacts[3])
					})
				})

				Context("When allowed time is in a future day", func() {
					BeforeEach(func() {
						notifier.GetNow = func() time.Time {
							return time.Unix(1441101600, 0) // 1 Сентябрь 2015 г. 15:00:00 (GMT +5)
						}
						err = notifier.ProcessEvent(event)
						Expect(err).ShouldNot(HaveOccurred())
					})
					It("should send notification at the beginning of allowed interval", func() {
						assertNotificationSent(event, time.Unix(1441134000, 0), contacts[2]) // 2 Сентябрь 2015 г. 00:00:00 (GMT +5)
					})
				})
			})
		})
	})

	Context("When many events arrive for the same trigger", func() {
		Context("When nobody is subscribed", func() {
			BeforeEach(func() {
				for event := range generateEvents(11, triggers[4].ID) {
					err = notifier.ProcessEvent(*event)
					testDb.saveEvent(triggers[4].ID, event)
					Expect(err).ShouldNot(HaveOccurred())
				}
			})

			It("should not enable throttling", func() {
				assertNotifications(triggers[4].ID, notifier.GetNow(), 0, -1, 11)
				assertThrottling(triggers[4].ID, time.Unix(0, 0))
			})
		})

		Context("When somebody is subscribed", func() {
			Context("When he has schedule and planned notification time is not allowed", func() {
				BeforeEach(func() {
					for event := range generateEvents(11, triggers[2].ID) {
						err = notifier.ProcessEvent(*event)
						testDb.saveEvent(triggers[2].ID, event)
						Expect(err).ShouldNot(HaveOccurred())
					}
				})
				It("should plan next notification time on allowed interval", func() {
					assertNotifications(triggers[2].ID, time.Unix(1441738800, 0), 0, -1, 11)
				})
			})

			Context("When he has schedule and planned notification time is allowed", func() {
				BeforeEach(func() {
					notifier.GetNow = func() time.Time {
						return time.Unix(1441134000, 0) //  2 Сентябрь 2015 г. 00:00:00 (GMT +5)
					}
					for event := range generateEvents(11, triggers[2].ID) {
						err = notifier.ProcessEvent(*event)
						testDb.saveEvent(triggers[2].ID, event)
						Expect(err).ShouldNot(HaveOccurred())
					}
				})
				It("should plan next notification time according to throttling rules", func() {
					assertNotifications(triggers[2].ID, time.Unix(1441134000, 0), 0, 9, 10)  //  2 Сентябрь 2015 г. 00:00:00 (GMT +5)
					assertNotifications(triggers[2].ID, time.Unix(1441135800, 0), 10, -1, 1) //  2 Сентябрь 2015 г. 00:30:00 (GMT +5)
					assertThrottling(triggers[2].ID, time.Unix(1441135800, 0))
				})
			})

			Context("When he has no schedule", func() {
				Context("When throttling limit approached", func() {
					BeforeEach(func() {
						for event := range generateEvents(10, triggers[0].ID) {
							err = notifier.ProcessEvent(*event)
							testDb.saveEvent(triggers[0].ID, event)
							Expect(err).ShouldNot(HaveOccurred())
						}
					})
					It("last notification should indicate throttling limit approaching", func() {
						notifications, err := testDb.getNotifications(0, -1)
						Expect(err).ShouldNot(HaveOccurred())
						for index, notification := range notifications {
							if index < len(notifications)-1 {
								Expect(notification.Throttled).To(Equal(false))
							} else {
								Expect(notification.Throttled).To(Equal(true))
							}
						}
					})
					It("should plan next notification time immediately", func() {
						assertNotifications(triggers[0].ID, notifier.GetNow(), 0, -1, 11)
					})
				})
				Context("When throttling limit reached", func() {
					BeforeEach(func() {
						for event := range generateEvents(11, triggers[0].ID) {
							err = notifier.ProcessEvent(*event)
							testDb.saveEvent(triggers[0].ID, event)
							Expect(err).ShouldNot(HaveOccurred())
						}
					})
					It("should plan next notification time according to throttling rules", func() {
						assertNotifications(triggers[0].ID, notifier.GetNow(), 0, 9, 10)
						assertNotifications(triggers[0].ID, notifier.GetNow().Add(30*time.Minute), 10, -1, 1)
						assertThrottling(triggers[0].ID, notifier.GetNow().Add(30*time.Minute))
					})
				})
				Context("When throttling limit reached, but subscription disable throttling", func() {
					BeforeEach(func() {
						for event := range generateEvents(11, triggers[5].ID) {
							err = notifier.ProcessEvent(*event)
							testDb.saveEvent(triggers[5].ID, event)
							Expect(err).ShouldNot(HaveOccurred())
						}
					})
					It("should plan next notification time immediately", func() {
						assertNotifications(triggers[5].ID, notifier.GetNow(), 0, -1, 11)
						assertThrottling(triggers[5].ID, time.Unix(0, 0))
					})
				})
			})
		})

		Context("When multiple subscribers", func() {
			Context("When one has schedule and planned notification time is not allowed, but second has schedule and planned notification time is allowed", func() {
				Context("When throttling disabled", func() {
					BeforeEach(func() {
						for event := range generateEvents(11, triggers[6].ID) {
							err = notifier.ProcessEvent(*event)
							testDb.saveEvent(triggers[6].ID, event)
							Expect(err).ShouldNot(HaveOccurred())
						}
					})
					It("should plan next notification time on allowed interval", func() {
						assertNotifications(triggers[6].ID, notifier.GetNow(), 0, 10, 11)
						assertNotifications(triggers[6].ID, time.Unix(1441738800, 0), 11, 20, 11)
						assertThrottling(triggers[6].ID, notifier.GetNow().Add(30*time.Minute))
					})
				})
			})
		})

		Context("When build-in mail sender enabled", func() {
			var sender mail.Sender
			var m *gomail.Message
			BeforeEach(func() {
				sender = mail.Sender{
					FrontURI: "http://localhost",
					From:     "test@notifier",
					SMTPhost: "localhost",
					SMTPport: 25,
				}
				sender.SetLogger(log)
				events := make([]notifier.EventData, 0, 10)
				for event := range generateEvents(10, triggers[0].ID) {
					events = append(events, *event)
				}
				m = sender.MakeMessage(events, contacts[0], triggers[0], true)
			})

			It("make message with right headers", func() {
				Expect(m.GetHeader("From")[0]).To(Equal(sender.From))
				Expect(m.GetHeader("To")[0]).To(Equal(contacts[0].Value))
				m.WriteTo(os.Stdout)
			})
		})
	})
})

func assertProcessEvent(event notifier.EventData, expectError bool) {
	err := notifier.ProcessEvent(event)
	if expectError {
		Expect(err).Should(HaveOccurred())
	} else {
		Expect(err).ShouldNot(HaveOccurred())

	}
}

func assertNotificationSent(event notifier.EventData, timestamp time.Time, contact notifier.ContactData) {
	notification, err := testDb.getSingleNotification()
	Expect(err).ShouldNot(HaveOccurred())
	Expect(notification.Event.TriggerID).To(Equal(event.TriggerID))
	Expect(notification.Event.State).To(Equal(event.State))
	Expect(notification.Event.OldState).To(Equal(event.OldState))
	Expect(notification.Event.Metric).To(Equal(event.Metric))
	Expect(notification.SendFail).To(Equal(0))
	Expect(fmt.Sprintf("%s", time.Unix(notification.Timestamp, 0))).To(Equal(fmt.Sprintf("%s", timestamp)))
	Expect(notification.Contact.Value).To(Equal(contact.Value))
}

func assertThrottling(triggerID string, expected time.Time) {
	throttledTime, _ := testDb.conn.GetTriggerThrottlingTimestamps(triggerID)
	Expect(fmt.Sprintf("%s", throttledTime)).To(Equal(fmt.Sprintf("%s", expected)))
}

func assertNotifications(triggerID string, expected time.Time, from int, to int, count int) {
	notifications, err := testDb.getNotifications(from, to)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(len(notifications), count)
	for _, notification := range notifications {
		Expect(fmt.Sprintf("%s", time.Unix(notification.Timestamp, 0))).To(Equal(fmt.Sprintf("%s", expected)))
	}
}

func generateEvents(n int, triggerID string) chan *notifier.EventData {
	ch := make(chan *notifier.EventData)
	go func() {
		for i := 0; i < n; i++ {
			event := &notifier.EventData{
				Timestamp: notifier.GetNow().Unix(),
				Metric:    fmt.Sprintf("Metric number #%d", i),
				TriggerID: triggerID,
			}
			ch <- event
		}
		close(ch)
	}()
	return ch
}

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
