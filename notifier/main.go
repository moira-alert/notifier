package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/moira-alert/notifier"
	// 	"moira/notifier/kontur"
	"github.com/moira-alert/notifier/mail"
	"github.com/moira-alert/notifier/pushover"
	"github.com/moira-alert/notifier/script"
	"github.com/moira-alert/notifier/slack"
	"github.com/moira-alert/notifier/telegram"
	"github.com/moira-alert/notifier/twilio"

	"github.com/gosexy/to"
	"github.com/gosexy/yaml"
	"github.com/op/go-logging"
)

var (
	log            *logging.Logger
	config         notifier.Settings
	configFileName = flag.String("config", "/etc/moira/config.yml", "path to config file")
	printVersion   = flag.Bool("version", false, "Print current version and exit")

	Version = "latest"
)

type yamlSettings struct {
	file *yaml.Yaml
}

func (s *yamlSettings) Get(section, key string) string {
	return to.String(s.file.Get(section, key))
}

func (s *yamlSettings) GetInterface(section, key string) interface{} {
	return s.file.Get(section, key)
}

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
	if err := readSettings(*configFileName); err != nil {
		fmt.Printf("Can not read settings: %s \n", err.Error())
		os.Exit(1)
	}

	if err := configureLog(); err != nil {
		fmt.Printf("Can not configure log: %s \n", err.Error())
		os.Exit(1)
	}
	notifier.InitRedisDatabase()
	if err := configureSenders(); err != nil {
		log.Fatalf("Can not configure senders: %s", err.Error())
	}
	notifier.InitMetrics()

	shutdown := make(chan bool)
	var wg sync.WaitGroup
	run(notifier.FetchEvents, shutdown, &wg)
	run(notifier.FetchScheduledNotifications, shutdown, &wg)
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

	logLevel, err := logging.LogLevel(config.Get("notifier", "log_level"))
	if err != nil {
		logLevel = logging.DEBUG
	}
	logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}\t%{level}\t%{message}"))
	logFileName := config.Get("notifier", "log_file")
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
	logBackend.Color = (config.Get("notifier", "log_color") == "true")
	logging.SetBackend(logBackend)
	logging.SetLevel(logLevel, "notifier")

	notifier.SetLogger(log)

	return nil
}

func configureSenders() error {
	sendersList, ok := config.GetInterface("notifier", "senders").([]interface{})
	if ok == false {
		return fmt.Errorf("Failed parse senders")
	}
	for _, senderSettingsI := range sendersList {
		senderSettings := make(map[string]string)
		for k, v := range senderSettingsI.(map[interface{}]interface{}) {
			senderSettings[to.String(k)] = to.String(v)
		}
		senderSettings["front_uri"] = config.Get("front", "uri")
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

func readSettings(configFileName string) error {
	file, err := yaml.Open(configFileName)
	if err != nil {
		return fmt.Errorf("Can't read config file %s: %s", configFileName, err.Error())
	}
	config = &yamlSettings{file}
	notifier.SetSettings(config)

	return nil
}
