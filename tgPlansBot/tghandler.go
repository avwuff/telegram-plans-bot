package tgPlansBot

import (
	"context"
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strings"
	"sync"
)

type TGPlansBot struct {
	// the commands i will listen to
	cmds *tgCommands.CommandList

	// the salt value for my MD5 code
	saltValue string

	// my database connection
	db dbInterface.DBFeatures

	// my telegram bot interface
	tg TelegramBot

	// queue is used for keeping track of retry delay sends
	queue sync.Map
}

func (tgp *TGPlansBot) StartTG(ctx context.Context, salt string, dbMain dbInterface.DBFeatures, tg TelegramBot) {
	tgp.saltValue = salt
	tgp.db = dbMain
	tgp.tg = tg

	// Set up the initial set of commands and what each one does, and under which modes it is active.
	tgp.initCommands()

	// Set up the available commands in the bot
	tgp.setMyCommands()

	// Start listening
	tgp.tg.Listen(ctx, tgp.handleUpdate)
}

func (tgp *TGPlansBot) initCommands() {
	loc := localizer.FromLanguage("default") // not a real locale

	tgp.cmds = tgCommands.NewList()
	tgp.cmds.Add(tgCommands.Command{Command: "/start", Handler: tgp.startHandler, HelpText: loc.Sprintf("Create a new set of plans")})
	tgp.cmds.Add(tgCommands.Command{Command: "/help", Handler: tgp.helpHandler, HelpText: loc.Sprintf("Display the help message")})
	tgp.cmds.Add(tgCommands.Command{Command: "/feed", Handler: tgp.calendarFeed, HelpText: loc.Sprintf("Get a custom calendar feed")})
	tgp.cmds.Add(tgCommands.Command{Command: "/language", Handler: tgp.languageHandler, HelpText: loc.Sprintf("Change the language")})
	tgp.cmds.Add(tgCommands.Command{Command: "/setup", Handler: tgp.setupHandler, HelpText: loc.Sprintf("Start the Setup process")})
	tgp.cmds.Add(tgCommands.Command{Command: "/about", Handler: tgp.aboutHandler, HelpText: loc.Sprintf("Learn more about the bot")})
	tgp.cmds.Add(tgCommands.Command{Command: "/going", Handler: tgp.goingHandler, HelpText: loc.Sprintf("See a list of all events you RSVPd to")})
	tgp.cmds.Add(tgCommands.Command{Command: "/nearby", Handler: tgp.nearbyHandler, HelpText: loc.Sprintf("See nearby events listed in our public directory")})
	tgp.cmds.Add(tgCommands.Command{Command: "/nearbyfeed", Handler: tgp.nearbyFeedHandler, HelpText: loc.Sprintf("Get an ICS calendar feed of all nearby events")})
	tgp.cmds.Add(tgCommands.Command{Command: "/globalmsg", Handler: tgp.sendGlobalMessage, Private: true})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "moreinfo", Handler: tgp.goingMoreInfo})
	tgp.cmds.SetUnknown(tgp.unknownHandler)

	// These handlers respond to any message, as long as we are in the right mode.
	// SETTINGS
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETLANGUAGE, Handler: tgp.setLanguageHandler})

	// EVENTS
	tgp.initEventCommands()
	tgp.initSetupCommands()
	tgp.initUICommands()
	tgp.initGlobalMsgCommands()
	tgp.initGuestCommands()
	tgp.initDonateCommands()
	tgp.initNearbyCommands()

}

func (tgp *TGPlansBot) setMyCommands() {
	// Now add the commands to the command menu.
	base := tgp.cmds.BaseCommandList()
	// Do this for each language
	langs := localizer.GetLanguageChoicesISO639()
	for locale, isoCode := range langs {
		// get a localizer for this language
		loc := localizer.FromLanguage(locale)

		var cmdList []tgbotapi.BotCommand

		// build the command list
		for _, cmd := range base {
			if !cmd.Private {
				cmdList = append(cmdList, tgbotapi.BotCommand{
					Command:     cmd.Command,
					Description: loc.Sprintf(cmd.HelpText),
				})
			}
		}
		_, err := tgp.tg.SetMyCommands(tgbotapi.SetMyCommandsConfig{Commands: cmdList, LanguageCode: isoCode})
		if err != nil {
			log.Println(err)
		}
	}
}

func (tgp *TGPlansBot) startHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Are we doing one of the special inline starts?
	// Example:
	// /start SetGuestNames_8265bf4ef5c0b7afd8336d620fed2dee
	if strings.HasPrefix(text, GUEST_START_PREFIX) {
		tgp.handleGuestStart(usrInfo, msg, text)
		return
	}

	if strings.HasPrefix(text, DONATE_START_PREFIX) {
		tgp.handleDonateStart(usrInfo, msg, text)
		return
	}

	// Has the user completed the setup process?
	if !usrInfo.Prefs.SetupComplete {
		tgp.startSetup(usrInfo, msg)
		return
	}

	// Reset the user back to the default mode.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTNAME)
	tgp.quickReply(msg, usrInfo.Locale.Sprintf("Let's create some new plans.  First, send me the name of the event."))
}

// User wants to start the setup process again
func (tgp *TGPlansBot) setupHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	tgp.startSetup(usrInfo, msg)
}

func (tgp *TGPlansBot) helpHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Build the help message.
	base := tgp.cmds.BaseCommandList()
	txt := usrInfo.Locale.Sprintf("Here is a list of available commands:") + "\n\n"
	for _, cmd := range base {
		if !cmd.Private {
			txt += fmt.Sprintf("<b>%v</b> - %v\n", cmd.Command, usrInfo.Locale.Sprintf(cmd.HelpText))
		}
	}
	tgp.quickReply(msg, txt)
}

func (tgp *TGPlansBot) aboutHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Build the help message.
	txt := usrInfo.Locale.Sprintf(`The Furry Plans bot was created by 🐕‍🦺<b>Av</b> (www.avbrand.com)

Translations provided by:`) + ` 
<b>Deutsch</b>: Banane9
<b>Française Canadian</b>: Boof, Snarl
<b>Française</b>: Achorawl
` + usrInfo.Locale.Sprintf(`
This project is open source! Learn more at: https://github.com/avwuff/telegram-plans-bot`)
	tgp.quickReply(msg, txt)
}

func (tgp *TGPlansBot) unknownHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	_ = tgp.quickReply(msg, usrInfo.Locale.Sprintf("I don't understand that command. Send /help for help."))
}

func (tgp *TGPlansBot) handleUpdate(update tgbotapi.Update) {

	// What kind of update is it?
	if update.Message != nil {
		tgp.handleMsg(update.Message)
	} else if update.CallbackQuery != nil {
		tgp.handleCallback(update.CallbackQuery)
	} else if update.InlineQuery != nil {
		tgp.handleInline(update.InlineQuery)
	}

}

func (tgp *TGPlansBot) handleMsg(msg *tgbotapi.Message) {
	// Get the mode the user is in
	usrInfo := userManager.Get(msg.Chat.ID)

	// Let the command list handler handle it
	tgp.cmds.Process(usrInfo, msg)
}

func (tgp *TGPlansBot) handleCallback(callback *tgbotapi.CallbackQuery) {
	// Get the mode the user is in
	usrInfo := userManager.Get(callback.From.ID)

	// See if this callback is one of the ones we can handle.
	tgp.cmds.ProcessCallback(usrInfo, callback)
}

func (tgp *TGPlansBot) quickReply(msg *tgbotapi.Message, text string) error {
	mObj := tgbotapi.NewMessage(msg.Chat.ID, text)
	mObj.ParseMode = ParseModeHtml
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	_, err := tgp.tg.Send(mObj)
	if err != nil {
		return err
	}
	return nil

}
