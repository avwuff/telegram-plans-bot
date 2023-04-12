package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strings"
	"time"
)

func initSetupCommands(cmds *tgCommands.CommandList) {
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_LANGUAGE, Handler: setup_setLanguage})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_TIMEZONE, Handler: setup_setTimeZone})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_POLICY, Handler: setup_setPolicy})

}

// Handling of the setup process.
func startSetup(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_LANGUAGE)

	// Note that this phrase is not translated since we don't know the user's language yet.
	mObj := tgbotapi.NewMessage(msg.Chat.ID, "Hello!  I'm the Furry Plans Bot, version 2.0!\n\nI see this is your first time.  Let me take you through the setup process first.\n\nWhat language do you speak?")
	mObj.ReplyMarkup = localizer.GetLanguageChoices()

	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func setup_setLanguage(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if this is one of the languages we support.
	lang, err := localizer.FromLanguageName(text)
	if err != nil {
		quickReply(tg, msg, "Language not found")
		return
	}

	// Set the language.
	usrInfo.Prefs.Language = lang
	dbHelper.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "language")

	// Replace the localizer since the language has been changed
	usrInfo.Locale = localizer.FromLanguage(lang)

	// Go on to the next part.
	setup_askTimeZone(tg, usrInfo, msg)
}

func setup_askTimeZone(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_TIMEZONE)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf("In which time zone do you live?"))

	tzMap := localizer.GetTimeZoneChoicesMap()
	tzs := localizer.GetTimeZoneChoicesList()
	// Build the choices.
	var keyboard [][]tgbotapi.KeyboardButton
	for _, tzl := range tzs {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(fmt.Sprintf("%v: %v", tzl.DisplayText, time.Now().In(tzMap[tzl.Key]).Format("15:04"))),
		))
	}

	mObj.ReplyMarkup = tgbotapi.ReplyKeyboardMarkup{
		ResizeKeyboard: true,
		Keyboard:       keyboard,
	}

	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func setup_setTimeZone(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Get the text before the ":"
	s := strings.Split(text, ":")
	tz := s[0]
	tzs := localizer.GetTimeZoneChoicesMap()
	_, ok := tzs[tz]
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Specified Time Zone not found."))
		return
	}
	// Set the language.
	usrInfo.Prefs.TimeZone = tz
	dbHelper.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "time_zone")

	// Go on to the next part.
	setup_askPolicy(tg, usrInfo, msg)

}

func setup_askPolicy(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_POLICY)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(`Before you continue, please read and accept our privacy policy.

https://telegra.ph/Furry-Plans-Bot-Privacy-Policy-06-29

You'll find the instructions on how to continue at the bottom of that page.`))
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func setup_setPolicy(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	if strings.ToLower(text) != "i accept" {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Please check the policy again to see how to finish."))
		return
	}

	usrInfo.Prefs.SetupComplete = true
	dbHelper.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "setup_complete")

	usrInfo.SetMode(userManager.MODE_DEFAULT)

	// All done!
	quickReply(tg, msg, usrInfo.Locale.Sprintf("Thanks!  You're all set to start using the Furry Plans Bot!  Type /start to create a new set of plans."))
}
