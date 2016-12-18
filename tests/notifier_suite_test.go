package tests

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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

var (
	tcReport       = flag.Bool("teamcity", false, "enable TeamCity reporting format")
	useFakeDb      = flag.Bool("fakedb", true, "use fake db instead localhost real redis")
	log            *logging.Logger
	testDb         *testDatabase
	testConfig     *notifier.Config
	sendersRunning = false
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
		testConfig = &notifier.Config{
			Notifier: notifier.NotifierConfig{
				SenderTimeout:    "0s10ms",
				ResendingTimeout: "24:00",
				SelfState: notifier.SelfStateConfig{
					Enabled: "true",
					Contacts: []map[string]string{
						map[string]string{
							"type":  "admin-mail",
							"value": "admin@company.com",
						},
					},
					RedisDisconectDelay:     10,
					LastMetricReceivedDelay: 60,
					LastCheckDelay:          120,
				},
			},
		}
		notifier.SetSettings(testConfig)
		logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}\t%{level}\t%{message}"))
		logBackend := logging.NewLogBackend(os.Stdout, "", 0)
		logBackend.Color = false
		logging.SetBackend(logBackend)
		logging.SetLevel(logging.DEBUG, "notifier")
		now := notifier.GetNow()
		log.Debugf("Using now time: %s, %s", now, now.Weekday())
	})

	AfterEach(func() {
		stopSenders()
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
		notifier.RegisterSender(map[string]string{"type": "email"}, &badSender{})
		notifier.RegisterSender(map[string]string{"type": "slack"}, &timeoutSender{})
		sendersRunning = true
	})

	Context("Event pseudo tags", func() {
		Context("Progress", func() {
			It("Should contains progress tag", func() {
				event := notifier.EventData{
					State:    "OK",
					OldState: "WARN",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"OK", "WARN", "PROGRESS"}))
			})
		})
		Context("Degradation", func() {
			It("Should contains degradation tag", func() {
				event := notifier.EventData{
					State:    "WARN",
					OldState: "OK",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"WARN", "OK", "DEGRADATION"}))
			})
			It("Should contains degradation tag", func() {
				event := notifier.EventData{
					State:    "ERROR",
					OldState: "WARN",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"ERROR", "WARN", "DEGRADATION"}))
			})
			It("Should contains high degradation tag", func() {
				event := notifier.EventData{
					State:    "ERROR",
					OldState: "OK",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"ERROR", "OK", "HIGH DEGRADATION", "DEGRADATION"}))
			})
			It("Should contains high degradation tag", func() {
				event := notifier.EventData{
					State:    "NODATA",
					OldState: "ERROR",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"NODATA", "ERROR", "HIGH DEGRADATION", "DEGRADATION"}))
			})
		})
		Context("Non-weighted test tag", func() {
			It("Should contains test tag", func() {
				event := notifier.EventData{
					State:    "TEST",
					OldState: "TEST",
				}
				tags := event.GetPseudoTags()
				Expect(tags).To(Equal([]string{"TEST", "TEST"}))
			})
		})
	})

	Context("SelfCheck", func() {
		BeforeEach(func() {
			notifier.SelfCheckInterval = time.Millisecond * 10
		})

		Context("Config", func() {
			Context("Admin sender not registered", func() {
				It("Should not pass check without admin contact", func() {
					err := notifier.CheckSelfStateMonitorSettings()
					Expect(err).Should(HaveOccurred())
				})
			})
			Context("Admin sender registered", func() {
				var sender *adminSender
				BeforeEach(func() {
					sender = &adminSender{}
					notifier.RegisterSender(map[string]string{
						"type": "admin-mail",
					}, sender)
				})
				It("Should pass check", func() {
					err := notifier.CheckSelfStateMonitorSettings()
					Expect(err).ShouldNot(HaveOccurred())
				})
				Context("Redis disconnected", func() {
					var (
						shutdown chan bool
						wg       sync.WaitGroup
					)

					BeforeEach(func() {
						shutdown = make(chan bool)
						offset := int64(10)
						notifier.GetNow = func() time.Time {
							return time.Now().Add(time.Second * time.Duration(atomic.LoadInt64(&offset)))
						}
						wg.Add(2)
						go notifier.SelfStateMonitor(shutdown, &wg)
						go func() {
							defer wg.Done()
							for {
								select {
								case <-shutdown:
									return
								case <-time.After(time.Millisecond):
									atomic.AddInt64(&offset, 10)
								}
							}
						}()
					})

					AfterEach(func() {
						close(shutdown)
						wg.Wait()
					})

					It("Should notify admin", func() {
						begin := time.Now()
						for {
							sender.mutex.Lock()
							if sender.lastEvents != nil || begin.Add(time.Second).Before(time.Now()) {
								sender.mutex.Unlock()
								break
							}
							sender.mutex.Unlock()
							time.Sleep(time.Millisecond * 10)
						}
						sender.mutex.Lock()
						Expect(sender.lastEvents).ToNot(BeNil())
						Expect(sender.lastEvents[0].Metric).To(Equal("Moira-Cache does not received new metrics"))
						sender.mutex.Unlock()
					})
				})
			})
		})
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
				stopSenders()
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
					stopSenders()
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
				stopSenders()
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
						assertNotifications(triggers[0].ID, notifier.GetNow(), 0, -1, 10)
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
						assertNotifications(triggers[6].ID, time.Unix(1441738800, 0), 11, 21, 11)
						assertThrottling(triggers[6].ID, notifier.GetNow().Add(30*time.Minute))
					})
				})
			})
			Context("When subscribtions has the same contact", func() {
				BeforeEach(func() {
					for event := range generateEvents(10, triggers[7].ID) {
						err = notifier.ProcessEvent(*event)
						testDb.saveEvent(triggers[7].ID, event)
						Expect(err).ShouldNot(HaveOccurred())
					}
				})
				It("should not duplicate notifications", func() {
					assertNotifications(triggers[7].ID, notifier.GetNow(), 0, -1, 10)
					assertThrottling(triggers[7].ID, time.Unix(0, 0))
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
				for event := range generateTestEvents(10, triggers[0].ID) {
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

	Context("Initialization methods", func() {
		config := notifier.RedisConfig{}
		db := notifier.InitRedisDatabase(config)
		It("should create connector pool", func() {
			Expect(db).ShouldNot(BeNil())
		})
		It("should return error when trying to fake connect", func() {
			_, err := db.Pool.Dial()
			Expect(err).Should(HaveOccurred())
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
	Expect(len(notifications)).To(Equal(count))
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

func stopSenders() {
	if sendersRunning {
		notifier.StopSenders()
	}
	sendersRunning = false
}
