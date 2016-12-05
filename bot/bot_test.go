package bot

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gmlexx/redigomock"
	"github.com/moira-alert/notifier"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	logging "github.com/op/go-logging"
	"github.com/tucnak/telebot"
)

var (
	b          bot
	tcReport   = flag.Bool("teamcity", false, "enable TeamCity reporting format")
	useFakeDb  = flag.Bool("fakedb", true, "use fake db instead localhost real redis")
	testDb     *testDatabase
	testConfig *notifier.Config
)

func TestBot(t *testing.T) {
	flag.Parse()

	RegisterFailHandler(Fail)
	if *tcReport {
		RunSpecsWithCustomReporters(t, "Bot Suite", []Reporter{reporters.NewTeamCityReporter(os.Stdout)})
	} else {
		RunSpecs(t, "Bot Suite")
	}
}

var _ = Describe("Bot", func() {
	BeforeSuite(func() {
		logger, _ = logging.GetLogger("bot")

		logBackend := logging.NewLogBackend(os.Stdout, "", 0)

		// Only errors and more severe messages should be sent to backend1
		backendLeveled := logging.AddModuleLevel(logBackend)
		backendLeveled.SetLevel(logging.DEBUG, "")
		logBackend.Color = false
		logger.SetBackend(backendLeveled)
		logging.SetLevel(logging.DEBUG, "bot")
		logger.Debugf("Logger initialized")
	})
	BeforeEach(func() {
		testDb = &testDatabase{&notifier.DbConnector{}}
		c := redigomock.NewFakeRedis()
		testDb.conn.Pool = &redis.Pool{
			MaxIdle:     3,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				return c, nil
			},
		}
		testDb.init()
		notifier.SetDb(testDb.conn)

		b = bot{
			key:      "test-key",
			telebot:  &FakeTelebot{},
			messages: make(chan telebot.Message),
			db:       testDb.conn,
		}
	})
	Context("Recepient", func() {

		username := "test_username"
		r := recipient{username}

		destination := r.Destination()
		It("Should return uid as destination", func() {
			Expect(username).To(Equal(destination))
		})
	})

	Context("Message handling", func() {

		message := telebot.Message{}

		Context("Group", func() {
			It("Should return no error", func() {
				chat := telebot.Chat{
					Type: "group",
				}
				message.Chat = chat
				err := b.handleMessage(message)
				Expect(err).To(BeNil())
			})
		})

		Context("Private", func() {
			It("Should return no error", func() {
				chat := telebot.Chat{
					Type: "private",
				}
				message.Chat = chat
				message.Text = "/start"
				err := b.handleMessage(message)
				Expect(err).To(BeNil())
			})
		})

	})

})
