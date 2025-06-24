package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

func (tgp *TGPlansBot) initNearbyCommands() {
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_LISTNEARBY, Handler: tgp.nearby_List})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_LISTNEARBYFEED, Handler: tgp.nearby_ListFeed})
}

// nearbyHandler lists out all events near you that are listed in the public directory
func (tgp *TGPlansBot) nearbyHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Set this user into the mode where they are changing languages.
	usrInfo.SetMode(userManager.MODE_LISTNEARBY)

	txt := usrInfo.Locale.Sprintf("Check out the events happening near you!\n\nSend me a location pin using the 📎 menu for a list of public events within 500 miles of you.")
	mObj := tgbotapi.NewMessage(msg.Chat.ID, txt)

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}
func (tgp *TGPlansBot) nearby_List(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	if msg.Location == nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please send a Location via Telegram's 'Send Location' feature.  Check inside the 📎 menu."))
		return
	}

	// Find all public events that are near this pin.
	events, err := tgp.db.NearbyFeed(msg.Location.Latitude, msg.Location.Longitude, 800)
	if err != nil {
		tgp.quickReply(msg, err.Error())
		return
	}

	if len(events) == 0 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Unfortunately, no nearby events found.  Isn't this a good opportunity to /start one?"))
		return
	}

	txt := usrInfo.Locale.Sprintf("Check out these events in your local area:\n\n")

	var buttons [][]tgbotapi.InlineKeyboardButton

	// List out the events
	for _, event := range events {

		btn := helpers.StripHtmlRegex(event.Name()) + " - " + usrInfo.Locale.FormatDateForLocale(event.DateTime())

		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(btn, fmt.Sprintf("moreinfo:%v", event.ID())))
		buttons = append(buttons, row)
	}

	mObj := tgbotapi.NewMessage(msg.Chat.ID, txt)
	mObj.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}

// nearbyFeedHandler prepares a feed of nearby events
func (tgp *TGPlansBot) nearbyFeedHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Set this user into the mode where they are changing languages.
	usrInfo.SetMode(userManager.MODE_LISTNEARBYFEED)

	txt := usrInfo.Locale.Sprintf("Get a feed of the events happening near you!\n\nSend me a location pin using the 📎 menu for a continually updating feed of events within 500 miles of you.")
	mObj := tgbotapi.NewMessage(msg.Chat.ID, txt)

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) nearby_ListFeed(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	if msg.Location == nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please send a Location via Telegram's 'Send Location' feature.  Check inside the 📎 menu."))
		return
	}

	// make the feed URL:

	// TODO: This should not be a hard-coded URL
	ICalURL := fmt.Sprintf("https://plansbot.avbrand.com/feed/nearby/%v/%v/plans.ics", msg.Location.Latitude, msg.Location.Longitude)
	tgp.quickReply(msg, usrInfo.Locale.Sprintf(`Cool, here is an iCal feed of all the events happening within 500 miles of you:

%v

You can add this feed URL to your Google Calendar or Outlook. Any events that are created will appear in the feed, and stay up to date!

Chheck out the /feed command also for a feed of events you're attending.'`, ICalURL))

}
