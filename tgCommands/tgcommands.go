package tgCommands

import (
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

type CommandHandler func(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string)
type CallbackHandler func(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery)

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
	commands []*Command
	cblist   []*Callback
	unknown  CommandHandler
}

func NewList() *CommandList {
	return &CommandList{}
}

func (c *CommandList) Add(command Command) {
	c.commands = append(c.commands, &command)
}

func (c *CommandList) AddCB(cb Callback) {
	c.cblist = append(c.cblist, &cb)
}

func (c *CommandList) SetUnknown(handler CommandHandler) {
	c.unknown = handler
}

// BaseCommandList will return the base commands for the help menu and the main command commands
func (c *CommandList) BaseCommandList() []Command {
	var out []Command
	for _, cmd := range c.commands {
		if cmd.Mode == userManager.MODE_DEFAULT &&
			cmd.Underscore == false {
			out = append(out, *cmd)
		}
	}
	return out
}

func (c *CommandList) Process(usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {
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
	for _, cmd := range c.commands {
		// Normal commands
		if !cmd.Underscore {
			if (cmd.Command == sp[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
				cmd.Handler(usrInfo, msg, text)
				return
			}
		} else {
			// underscore command
			if (cmd.Command == sp2[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
				cmd.Handler(usrInfo, msg, text2)
				return
			}
		}

		// Full text commands
		if (cmd.Command == "") && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode) {
			cmd.Handler(usrInfo, msg, msg.Text)
			return
		}
	}

	if c.unknown != nil {
		c.unknown(usrInfo, msg, msg.Text)
	}
}

func (c *CommandList) ProcessCallback(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Split the msg by spaces
	if cb.Data == "" {
		return
	}
	sp := strings.Split(cb.Data, ":")

	// See if this is one of our current commands
	for _, cmd := range c.cblist {
		// Normal commands
		if (cmd.DataPrefix == sp[0]) && (cmd.Mode == 0 || cmd.Mode == usrInfo.Eph.UserMode || cmd.Public) {
			cmd.Handler(usrInfo, cb)
			return
		}
	}
}
