package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moira-alert/notifier"
	logging "github.com/op/go-logging"
	"github.com/tucnak/telebot"
)

type bot struct {
	key      string
	telebot  *telebot.Bot
	messages chan telebot.Message
	db       notifier.Database
}
type recipient struct {
	uid string
}

func (r recipient) Destination() string {
	return r.uid
}

// Sender sends message
type Sender interface {
	Send(login, message string) error
}

// Starter starts bot
type Starter interface {
	Start() error
}

// Bot implements bot
type Bot interface {
	Sender
	Starter
}

var (
	logger *logging.Logger
)

// StartBot start a bot
func StartBot(key string, log *logging.Logger, db notifier.Database) Bot {
	logger = log
	messages := make(chan telebot.Message)

	api := &bot{
		key:      key,
		db:       db,
		messages: messages,
	}
	go api.Start()

	return api
}

func (b *bot) Start() error {
	var err error
	b.telebot, err = telebot.NewBot(b.key)
	if !b.db.NotifierRegistered() {
		b.db.RegisterNotifier()
		b.telebot.Listen(b.messages, 1*time.Second)

		for {
			message := <-b.messages
			if err = b.handleMessage(message); err != nil {
				logger.Errorf("Error sending message: %s", err)
			}
		}
	}
	return err
}

func (b *bot) Send(login, message string) error {
	uid, err := b.db.GetUsernameID(login)
	if err != nil {
		return err
	}
	logger.Debugf("Uid received: %s", uid)
	return b.telebot.SendMessage(recipient{uid}, message, nil)
}

func (b *bot) handleMessage(message telebot.Message) error {
	id := strconv.FormatInt(message.Chat.ID, 10)
	title := message.Chat.Title
	userTitle := strings.Trim(fmt.Sprintf("%s %s", message.Sender.FirstName, message.Sender.LastName), " ")
	username := message.Chat.Username
	chatType := message.Chat.Type
	switch {
	case chatType == "private" && message.Text == "/start":
		if username == "" {
			b.telebot.SendMessage(message.Chat, "Username is empty. Please add username in Telegram.", nil)
		} else {
			logger.Debugf("Start received: %s", userTitle)
			err := b.db.SetUsernameID("@"+username, id)
			if err != nil {
				return err
			}
			b.telebot.SendMessage(message.Chat, fmt.Sprintf("Okay, %s, your id is %s", userTitle, id), nil)
		}
	case chatType == "supergroup" || chatType == "group":
		logger.Debugf("Added to %s: %s", chatType, title)
		err := b.db.SetUsernameID(title, id)
		if err != nil {
			return err
		}
		b.telebot.SendMessage(message.Chat, fmt.Sprintf("Hi, all!\nI will send alerts in this group (%s).", title), nil)
	default:
		b.telebot.SendMessage(message.Chat, "I don't understand you :(", nil)
	}
	logger.Debugf("Message received: %v", message)
	return nil
}
