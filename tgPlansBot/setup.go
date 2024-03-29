package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/zsefvlol/timezonemapper"
	"log"
	"strings"
	"time"
)

func (tgp *TGPlansBot) initSetupCommands() {
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_LANGUAGE, Handler: tgp.setup_setLanguage})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_TIMEZONE, Handler: tgp.setup_setTimeZone})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_SETUP_POLICY, Handler: tgp.setup_setPolicy})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "policy", Mode: userManager.MODE_SETUP_POLICY, Handler: tgp.setup_setPolicyCB})

}

// Handling of the setup process.
func (tgp *TGPlansBot) startSetup(usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_LANGUAGE)

	// Note that this phrase is not translated since we don't know the user's language yet.
	mObj := tgbotapi.NewMessage(msg.Chat.ID, "Hello!  I'm the Furry Plans Bot, version 2.0!\n\nI see this is your first time.  Let me take you through the setup process first.\n\nWhat language do you speak?")
	mObj.ReplyMarkup = localizer.GetLanguageChoices()

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) setup_setLanguage(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if this is one of the languages we support.
	lang, err := localizer.FromLanguageName(text)
	if err != nil {
		tgp.quickReply(msg, "Language not found")
		return
	}

	// Set the language.
	usrInfo.Prefs.Language = lang
	tgp.db.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "language")

	// Replace the localizer since the language has been changed
	usrInfo.Locale = localizer.FromLanguage(lang)

	// Go on to the next part.
	tgp.setup_askTimeZone(usrInfo, msg)
}

func (tgp *TGPlansBot) setup_askTimeZone(usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_TIMEZONE)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(`In which time zone do you live?
You can either send one of the time zones on <a href="https://github.com/Lewington-pitsos/golang-time-locations">this</a> list,
or you can send your current 📍 Location by clicking the paperclip and choosing Location.`))
	mObj.LinkPreviewOptions.IsDisabled = true
	mObj.ParseMode = ParseModeHtml

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

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) setup_setTimeZone(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	var tz string
	// Did they send a location?
	if msg.Location != nil {
		// Convert this location to a time zone.
		tz = timezonemapper.LatLngToTimezoneString(msg.Location.Latitude, msg.Location.Longitude)
		if tz == "" {
			tgp.quickReply(msg, usrInfo.Locale.Sprintf("Specified Time Zone not found."))
			return
		}

		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Selected: %v", tz))

	} else {

		// Get the text before the ":"
		s := strings.Split(text, ":")
		tz = s[0]
		tzs := localizer.GetTimeZoneChoicesMap()
		_, ok := tzs[tz]
		if !ok {
			tgp.quickReply(msg, usrInfo.Locale.Sprintf("Specified Time Zone not found.  Please choose a time zone from the list.  If the list is not appearing, please click the button that looks like four boxes in the text entry field to bring up the choices."))
			return
		}
	}
	// Set the language.
	usrInfo.Prefs.TimeZone = tz
	tgp.db.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "time_zone")

	// Go on to the next part.
	tgp.setup_askPolicy(usrInfo, msg)

}

func (tgp *TGPlansBot) setup_askPolicy(usrInfo *userManager.UserInfo, msg *tgbotapi.Message) {

	usrInfo.SetMode(userManager.MODE_SETUP_POLICY)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(`Before you continue, please read and accept our privacy policy.

https://telegra.ph/Furry-Plans-Bot-Privacy-Policy-06-29`))
	var buttons [][]tgbotapi.InlineKeyboardButton

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(usrInfo.Locale.Sprintf("I accept"), "policy:accept"))
	buttons = append(buttons, row)

	mObj.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) setup_setPolicy(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	if strings.ToLower(text) != "i accept" {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please check the policy again to see how to finish."))
		return
	}

	usrInfo.Prefs.SetupComplete = true
	tgp.db.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "setup_complete")

	usrInfo.SetMode(userManager.MODE_DEFAULT)

	// All done!
	tgp.quickReply(msg, usrInfo.Locale.Sprintf("Thanks!  You're all set to start using the Furry Plans Bot!  Type /start to create a new set of plans."))

}

func (tgp *TGPlansBot) setup_setPolicyCB(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {

	usrInfo.Prefs.SetupComplete = true
	tgp.db.SavePrefs(cb.Message.Chat.ID, usrInfo.Prefs, "setup_complete")

	usrInfo.SetMode(userManager.MODE_DEFAULT)

	// All done!
	tgp.quickReply(cb.Message, usrInfo.Locale.Sprintf("Thanks!  You're all set to start using the Furry Plans Bot!  Type /start to create a new set of plans."))
}
