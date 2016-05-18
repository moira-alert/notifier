package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/moira-alert/notifier"
	"gopkg.in/yaml.v2"
	// 	"moira/notifier/kontur"
	"github.com/moira-alert/notifier/mail"
	"github.com/moira-alert/notifier/pushover"
	"github.com/moira-alert/notifier/script"
	"github.com/moira-alert/notifier/slack"
	"github.com/moira-alert/notifier/telegram"
	"github.com/moira-alert/notifier/twilio"
	"github.com/op/go-logging"
)

var (
	log            *logging.Logger
	config         *notifier.Config
	configFileName = flag.String("config", "/etc/moira/config.yml", "path to config file")
	printVersion   = flag.Bool("version", false, "Print current version and exit")

	Version = "latest"
)

type worker func(chan bool, *sync.WaitGroup)

func run(worker worker, shutdown chan bool, wg *sync.WaitGroup) {
	wg.Add(1)
	go worker(shutdown, wg)
}

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Printf("Moira notifier version: %s\n", Version)
		os.Exit(0)
	}
	var err error
	if config, err = readSettings(*configFileName); err != nil {
		fmt.Printf("Can not read settings: %s \n", err.Error())
		os.Exit(1)
	}
	notifier.SetSettings(config)
	if err := configureLog(); err != nil {
		fmt.Printf("Can not configure log: %s \n", err.Error())
		os.Exit(1)
	}
	if err := configureSenders(); err != nil {
		log.Fatalf("Can not configure senders: %s", err.Error())
	}
	if err := notifier.CheckSelfStateMonitorSettings(); err != nil {
		log.Fatalf("Can't configure self state monitor: %s", err.Error())
	}
	notifier.InitRedisDatabase()
	notifier.InitMetrics()

	shutdown := make(chan bool)
	var wg sync.WaitGroup
	run(notifier.FetchEvents, shutdown, &wg)
	run(notifier.FetchScheduledNotifications, shutdown, &wg)
	if config.Notifier.SelfState.Enabled == "true" {
		run(notifier.SelfStateMonitor, shutdown, &wg)
	} else {
		log.Debugf("Moira Self State Monitoring disabled")
	}

	log.Infof("Moira Notifier Started. Version: %s", Version)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Info(fmt.Sprint(<-ch))
	close(shutdown)
	wg.Wait()
	log.Infof("Moira Notifier Stopped. Version: %s", Version)
}

func configureLog() error {
	var err error
	log, err = logging.GetLogger("notifier")
	if err != nil {
		return fmt.Errorf("Can't initialize logger: %s", err.Error())
	}
	var logBackend *logging.LogBackend
	logLevel, err := logging.LogLevel(config.Notifier.LogLevel)
	if err != nil {
		logLevel = logging.DEBUG
	}
	logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}\t%{level}\t%{message}"))
	logFileName := config.Notifier.LogFile
	if logFileName == "stdout" || logFileName == "" {
		logBackend = logging.NewLogBackend(os.Stdout, "", 0)
	} else {
		logDir := filepath.Dir(logFileName)
		if err := os.MkdirAll(logDir, 755); err != nil {
			return fmt.Errorf("Can't create log directories %s: %s", logDir, err.Error())
		}
		logFile, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("Can't open log file %s: %s", logFileName, err.Error())
		}
		logBackend = logging.NewLogBackend(logFile, "", 0)
	}
	logBackend.Color = config.Notifier.LogColor == "true"
	logging.SetBackend(logBackend)
	logging.SetLevel(logLevel, "notifier")
	notifier.SetLogger(log)
	return nil
}

func configureSenders() error {
	for _, senderSettings := range config.Notifier.Senders {
		senderSettings["front_uri"] = config.Front.URI
		switch senderSettings["type"] {
		case "pushover":
			if err := notifier.RegisterSender(senderSettings, &pushover.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "slack":
			if err := notifier.RegisterSender(senderSettings, &slack.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "mail":
			if err := notifier.RegisterSender(senderSettings, &mail.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "script":
			if err := notifier.RegisterSender(senderSettings, &script.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "telegram":
			if err := notifier.RegisterSender(senderSettings, &telegram.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "twilio sms":
			if err := notifier.RegisterSender(senderSettings, &twilio.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		case "twilio voice":
			if err := notifier.RegisterSender(senderSettings, &twilio.Sender{}); err != nil {
				log.Fatalf("Can not register sender %s: %s", senderSettings["type"], err)
			}
		// case "email":
		// 	if err := notifier.RegisterSender(senderSettings, &kontur.MailSender{}); err != nil {
		// 	}
		// case "phone":
		// 	if err := notifier.RegisterSender(senderSettings, &kontur.SmsSender{}); err != nil {
		// 	}
		default:
			return fmt.Errorf("Unknown sender type [%s]", senderSettings["type"])
		}
	}
	return nil
}

func readSettings(configFileName string) (*notifier.Config, error) {
	config := &notifier.Config{
		Redis: notifier.RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
		Front: notifier.FrontConfig{
			URI: "http://localhost",
		},
		Graphite: notifier.GraphiteConfig{
			URI:      "localhost:2003",
			Prefix:   "DevOps.Moira",
			Interval: 60,
		},
		Notifier: notifier.NotifierConfig{
			LogFile:          "stdout",
			LogLevel:         "debug",
			LogColor:         "false",
			SenderTimeout:    "10s0ms",
			ResendingTimeout: "24:00",
			SelfState: notifier.SelfStateConfig{
				Enabled:                 "false",
				RedisDisconectDelay:     300,
				LastMetricReceivedDelay: 300,
				LastCheckDelay:          300,
				CkeckPeriod:             180,
			},
		},
	}
	configYaml, err := ioutil.ReadFile(configFileName)
	if err != nil {
		return nil, fmt.Errorf("Can't read file [%s] [%s]", configFileName, err.Error())
	}
	err = yaml.Unmarshal([]byte(configYaml), &config)
	if err != nil {
		return nil, fmt.Errorf("Can't parse config file [%s] [%s]", configFileName, err.Error())
	}
	return config, nil
}
