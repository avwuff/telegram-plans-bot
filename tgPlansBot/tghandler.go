package tgPlansBot

import (
	"context"
	"fmt"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

var cmds *tgCommands.CommandList

var saltValue string

func StartTG(ctx context.Context, salt string) {
	saltValue = salt

	// Create the tgWrapper object
	tg := initTg()

	// Set up the initial set of commands and what each one does, and under which modes it is active.
	initCommands()

	// Set up the available commands in the bot
	setMyCommands(tg)

	// Start listening
	go tg.Listen(ctx, handleUpdate)
}

func initCommands() {
	loc := localizer.FromLanguage("default") // not a real locale

	cmds = tgCommands.NewList()
	cmds.Add(tgCommands.Command{Command: "/start", Handler: startHandler, HelpText: loc.Sprintf("Create a new set of plans")})
	cmds.Add(tgCommands.Command{Command: "/help", Handler: helpHandler, HelpText: loc.Sprintf("Display the help message")})
	cmds.Add(tgCommands.Command{Command: "/feed", Handler: calendarFeed, HelpText: loc.Sprintf("Get a custom calendar feed")})
	cmds.Add(tgCommands.Command{Command: "/language", Handler: languageHandler, HelpText: loc.Sprintf("Change the language")})
	cmds.Add(tgCommands.Command{Command: "/setup", Handler: setupHandler, HelpText: loc.Sprintf("Start the Setup process")})
	cmds.Add(tgCommands.Command{Command: "/about", Handler: aboutHandler, HelpText: loc.Sprintf("Learn more about the bot")})
	cmds.SetUnknown(unknownHandler)

	// These handlers respond to any message, as long as we are in the right mode.
	// SETTINGS
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETLANGUAGE, Handler: setLanguageHandler})

	// EVENTS
	initEventCommands(cmds)
	initSetupCommands(cmds)
	initUICommands(cmds)

}

func setMyCommands(tg *tgWrapper.Telegram) {
	// Now add the commands to the command menu.
	base := cmds.BaseCommandList()
	// Do this for each language
	langs := localizer.GetLanguageChoicesISO639()
	for locale, isoCode := range langs {
		// get a localizer for this language
		loc := localizer.FromLanguage(locale)

		var cmdList []tgbotapi.BotCommand

		// build the command list
		for _, cmd := range base {
			cmdList = append(cmdList, tgbotapi.BotCommand{
				Command:     cmd.Command,
				Description: loc.Sprintf(cmd.HelpText),
			})
		}
		_, err := tg.SetMyCommands(tgbotapi.SetMyCommandsConfig{Commands: cmdList, LanguageCode: isoCode})
		if err != nil {
			log.Println(err)
		}
	}
}

func startHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Has the user completed the setup process?
	if !usrInfo.Prefs.SetupComplete {
		startSetup(tg, usrInfo, msg)
		return
	}

	// Reset the user back to the default mode.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTNAME)
	quickReply(tg, msg, usrInfo.Locale.Sprintf("Let's create some new plans.  First, send me the name of the event."))
}

// User wants to start the setup process again
func setupHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	startSetup(tg, usrInfo, msg)
}

func helpHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Build the help message.
	base := cmds.BaseCommandList()
	txt := usrInfo.Locale.Sprintf("Here is a list of available commands:") + "\n\n"
	for _, cmd := range base {
		txt += fmt.Sprintf("<b>%v</b> - %v\n", cmd.Command, usrInfo.Locale.Sprintf(cmd.HelpText))
	}
	quickReply(tg, msg, txt)
}

func aboutHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Build the help message.
	txt := usrInfo.Locale.Sprintf(`The Furry Plans bot was created by 🐕‍🦺<b>Av</b> (www.avbrand.com)

Translations provided by:`) + usrInfo.Locale.Sprintf(` 
<b>Deutsch</b>: Banane9
<b>Française Canadian</b>: Boof, Snarl
<b>Française</b>: Achorawl
`) + usrInfo.Locale.Sprintf(`
This project is open source! Learn more at: github.com/avwuff/furryplansbot`)
	quickReply(tg, msg, txt)
}

func unknownHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	_ = quickReply(tg, msg, usrInfo.Locale.Sprintf("I don't understand that command. Send /help for help."))
}

func initTg() *tgWrapper.Telegram {
	tg := tgWrapper.New()
	err := tg.LoadKeyFromFile("token.txt")
	if err != nil {
		log.Fatal(err)
	}
	err = tg.Init()
	if err != nil {
		log.Fatal(err)
	}
	return tg
}

func handleUpdate(tg *tgWrapper.Telegram, update tgbotapi.Update) {

	// What kind of update is it?
	if update.Message != nil {
		handleMsg(tg, update.Message)
	} else if update.CallbackQuery != nil {
		handleCallback(tg, update.CallbackQuery)
	} else if update.InlineQuery != nil {
		handleInline(tg, update.InlineQuery)
	}

}

func handleMsg(tg *tgWrapper.Telegram, msg *tgbotapi.Message) {
	// Get the mode the user is in
	usrInfo := userManager.Get(msg.Chat.ID)

	// Let the command list handler handle it
	cmds.Process(tg, usrInfo, msg)
}

func handleCallback(tg *tgWrapper.Telegram, callback *tgbotapi.CallbackQuery) {
	// Get the mode the user is in
	usrInfo := userManager.Get(int64(callback.From.ID))

	// See if this callback is one of the ones we can handle.
	cmds.ProcessCallback(tg, usrInfo, callback)
}

func quickReply(tg *tgWrapper.Telegram, msg *tgbotapi.Message, text string) error {
	mObj := tgbotapi.NewMessage(msg.Chat.ID, text)
	mObj.ParseMode = tgWrapper.ParseModeHtml
	_, err := tg.Send(mObj)
	if err != nil {
		return err
	}
	return nil
}
