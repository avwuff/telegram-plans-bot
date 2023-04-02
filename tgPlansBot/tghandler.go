package tgPlansBot

import (
	"context"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
)

var cmds *tgCommands.CommandList

func StartTG(ctx context.Context) {
	// Create the tgWrapper object
	tg := initTg()

	// Set up the initial set of commands and what each one does, and under which modes it is active.
	initCommands()

	// Start listening
	go tg.Listen(ctx, handleUpdate)
}

func initCommands() {
	cmds = tgCommands.NewList()
	cmds.Add(tgCommands.Command{Command: "/start", Handler: startHandler})
	cmds.Add(tgCommands.Command{Command: "/help", Handler: helpHandler})
	cmds.Add(tgCommands.Command{Command: "/language", Handler: languageHandler})
	cmds.Add(tgCommands.Command{Command: "/list", Handler: listHandler})
	cmds.Add(tgCommands.Command{Command: "/404", Handler: unknownHandler})

	// These handlers respond to any message, as long as we are in the right mode.
	// SETTINGS
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETLANGUAGE, Handler: setLanguageHandler})

	// EVENTS
	initEventCommands(cmds)
}

func startHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Reset the user back to the default mode.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTNAME)
	// TODO: Check privacy policy

	quickReply(tg, msg, usrInfo.Locale.Sprintf("Hi! I'm the Furry Plans Bot.  Let's create some new plans.  First, send me the name of the event."))
}

func helpHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// TODO: This should be generated automatically based on the command list.
	quickReply(tg, msg, usrInfo.Locale.Sprintf(`Here is a list of available commands:
/start
/language`))
}

func listHandler(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	_ = quickReply(tg, msg, usrInfo.Locale.Sprintf("Here is a list of your events:"))
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
	}

}

func handleMsg(tg *tgWrapper.Telegram, msg *tgbotapi.Message) {
	// Get the mode the user is in
	usrInfo := userManager.Get(msg.Chat.ID)

	// Let the command list handler handle it
	cmds.Process(tg, usrInfo, msg)
}

func handleCallback(tg *tgWrapper.Telegram, callback *tgbotapi.CallbackQuery) {
	// Is this one of the buttons on the global messages?
	// Those get handled separately.
	// TODO

	// Get the mode the user is in
	usrInfo := userManager.Get(int64(callback.From.ID))

	cmds.ProcessCallback(tg, usrInfo, callback)
}

func quickReply(tg *tgWrapper.Telegram, msg *tgbotapi.Message, text string) error {
	mObj := tgbotapi.NewMessage(msg.Chat.ID, text)
	_, err := tg.Send(mObj)
	if err != nil {
		return err
	}
	return nil
}
