package bot

import (
	"time"

	"github.com/tucnak/telebot"
)

type FakeTelebot struct{}

func (b *FakeTelebot) Listen(messages chan telebot.Message, timeout time.Duration) {
	messages <- telebot.Message{}
	close(messages)
}
func (b *FakeTelebot) SendMessage(chat telebot.Recipient, message string, opts *telebot.SendOptions) error {
	return nil
}
