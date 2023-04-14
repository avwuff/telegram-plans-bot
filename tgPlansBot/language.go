package tgPlansBot

import (
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// languageHandler is for allowing the user to change their language.
func (tgp *TGPlansBot) languageHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Present them a choice of the available languages.
	mObj := tgbotapi.NewMessage(msg.Chat.ID, "Select Language")
	mObj.ReplyMarkup = localizer.GetLanguageChoices()

	// Set this user into the mode where they are changing languages.
	usrInfo.SetMode(userManager.MODE_SETLANGUAGE)

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// TODO get rid of this function
func (tgp *TGPlansBot) setLanguageHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if this is one of the languages we support.
	lang, err := localizer.FromLanguageName(text)
	if err != nil {
		tgp.quickReply(msg, "Language not found")
		return
	}

	// Set the language.
	usrInfo.Prefs.Language = lang
	tgp.db.SavePrefs(msg.Chat.ID, usrInfo.Prefs, "language")
	usrInfo.SetMode(userManager.MODE_DEFAULT)

	// Replace the localizer since the language has been changed
	usrInfo.Locale = localizer.FromLanguage(lang)

	// Note that this phrase will get translated.
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf("The language has been set to English."))
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}
