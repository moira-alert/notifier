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

const messenger = "telegram"

type Telebot interface {
	Listen(chan telebot.Message, time.Duration)
	SendMessage(telebot.Recipient, string, *telebot.SendOptions) error
}

type bot struct {
	key      string
	telebot  Telebot
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

// Listener starts bot
type Listener interface {
	Listen()
}

// Bot implements bot
type Bot interface {
	Sender
	Listener
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
	var err error
	api.telebot, err = telebot.NewBot(key)
	if err != nil {
		log.Warning("Fail to create bot", err)
	}
	if db.RegisterBotIfAlreadyNot(messenger) {
		go api.Listen()
	}
	return api
}

func (b *bot) Listen() {
	b.telebot.Listen(b.messages, 1*time.Second)

	for message := range b.messages {
		if err := b.handleMessage(message); err != nil {
			logger.Errorf("Error sending message: %s", err)
		}
	}
}

func (b *bot) Send(username, message string) error {
	uid, err := b.db.GetIDByUsername(messenger, username)
	if err != nil {
		return err
	}
	logger.Debugf("Uid received: %s", uid)
	return b.telebot.SendMessage(recipient{uid}, message, nil)
}

func (b *bot) handleMessage(message telebot.Message) error {
	var err error
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
			err = b.db.SetUsernameID(messenger, "@"+username, id)
			if err != nil {
				return err
			}
			b.telebot.SendMessage(message.Chat, fmt.Sprintf("Okay, %s, your id is %s", userTitle, id), nil)
		}
	case chatType == "supergroup" || chatType == "group":
		logger.Debugf("Added to %s: %s", chatType, title)
		fmt.Println(chatType, title)
		err = b.db.SetUsernameID(messenger, title, id)
		if err != nil {
			return err
		}
		b.telebot.SendMessage(message.Chat, fmt.Sprintf("Hi, all!\nI will send alerts in this group (%s).", title), nil)
	default:
		b.telebot.SendMessage(message.Chat, "I don't understand you :(", nil)
	}
	logger.Debugf("Message received: %v", message)
	return err
}
