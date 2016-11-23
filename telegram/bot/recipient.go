package bot

import "github.com/tucnak/telebot"

type recipient struct {
	uid string
}

// CreateRecipient creates an recipient struct
func CreateRecipient(uid string) telebot.Recipient {
	return recipient{uid}
}

func (r recipient) Destination() string {
	return r.uid
}
