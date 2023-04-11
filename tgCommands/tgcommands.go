package tgCommands

import (
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

type CommandHandler func(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string)
type CallbackHandler func(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery)

type Command struct {
	// The text of the command
	Command string

	// The mode in which the command is available.  Blank is available in all modes.
	Mode userManager.Mode

	// The handler that should be called when this command is found
	Handler CommandHandler

	// Command is followed by underscore, so it is all one word
	Underscore bool

	// HelpText is the text shown for help with this command
	HelpText string
}

type Callback struct {
	// The first bit of the callback data, before the first colon
	DataPrefix string

	// Public callbacks are buttons that appear on the main UI of the event
	// These don't require any user data, and work all the time.
	Public bool

	// The mode in which the command is available.  Blank is available in all modes.
	Mode userManager.Mode

	// The handler that should be called when this command is found
	Handler CallbackHandler
}

type CommandList struct {
	list   []*Command
	cblist []*Callback
}

func NewList() *CommandList {
	return &CommandList{}
}

func (c *CommandList) Add(command Command) {
	c.list = append(c.list, &command)
}

func (c *CommandList) AddCB(cb Callback) {
	c.cblist = append(c.cblist, &cb)
}

func (c *CommandList) Process(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {
	// Split the msg by spaces
	if msg.Text == "" {
		return
	}
	sp := strings.Split(msg.Text, " ")
	text := strings.Join(sp[1:], " ")

	// if we are sending a /edit_1234 style command, handle that too.
	sp2 := strings.Split(msg.Text, "_")
	text2 := strings.Join(sp2[1:], " ")

	// See if this is one of our current commands
	for _, cmd := range c.list {
		// Normal commands
		if !cmd.Underscore {
			if (cmd.Command == sp[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
				cmd.Handler(tg, usrInfo, msg, text)
				return
			}
		} else {
			// underscore command
			if (cmd.Command == sp2[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
				cmd.Handler(tg, usrInfo, msg, text2)
				return
			}
		}

		// Full text commands
		if (cmd.Command == "") && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
			cmd.Handler(tg, usrInfo, msg, msg.Text)
			return
		}
	}

	// No matching command?  Reply with an unknown.
	msg.Text = "/404"
	c.Process(tg, usrInfo, msg)
}

func (c *CommandList) ProcessCallback(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Split the msg by spaces
	if cb.Data == "" {
		return
	}
	sp := strings.Split(cb.Data, ":")

	// See if this is one of our current commands
	for _, cmd := range c.cblist {
		// Normal commands
		if (cmd.DataPrefix == sp[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode || cmd.Public) {
			cmd.Handler(tg, usrInfo, cb)
			return
		}
	}
}
