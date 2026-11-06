package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"log"
	"strconv"
	"strings"
)

// goingHandler lists out all the events you are going to.
func (tgp *TGPlansBot) goingHandler(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	events, err := tgp.db.CalendarFeed(msg.Chat.ID)
	if err != nil {
		tgp.quickReply(msg, err.Error())
		return
	}
	txt := usrInfo.Locale.Sprintf("You're ✅ planning or 🤔️ considering attending the following events.  Click an event for more info.\n\n")

	var buttons [][]tgbotapi.InlineKeyboardButton

	// List out the events
	for _, event := range events {

		state := ""
		switch event.GetCanAttend() {
		case dbInterface.CANATTEND_YES:
			if event.Suitwalk() {
				state = "🙋"
			} else {
				state = "✅"
			}

		case dbInterface.CANATTEND_MAYBE:
			state = "🤔️"
		case dbInterface.CANATTEND_SUITING:
			state = "🐕"
		case dbInterface.CANATTEND_PHOTOGRAPHER:
			state = "📷"
		}

		btn := helpers.StripHtmlRegex(state+event.Name()) + " - " + usrInfo.Locale.FormatDateForLocale(event.DateTime())

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

func (tgp *TGPlansBot) goingMoreInfo(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// An event was selected -- find out more info about it.
	// In this case, it is events we are attending, not events we created.
	// So, first make sure we're actually on the attending list for this one.
	// Find this event.

	data := strings.Split(cb.Data, ":")
	if len(data) < 2 {
		return
	}

	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		tgp.quickReply(cb.Message, usrInfo.Locale.Sprintf("Unable to parse event ID: %v", err))
		return
	}

	// Load the details about the event from the database.
	event, err := tgp.db.GetEvent(uint(eventId), -1)
	if err != nil {
		tgp.quickReply(cb.Message, usrInfo.Locale.Sprintf("Event not found"))
		return
	}

	// Make sure this is an event they are actually attending.
	if !event.AmIAttending(cb.Message.Chat.ID) {
		tgp.quickReply(cb.Message, usrInfo.Locale.Sprintf("Not an event you are attending."))
		// Only allow public events to be shown to people who aren't attending.
		isPublic, _, _ := event.Public()
		if !isPublic {
			return
		}
	}

	// Now display the event UI here, allowing the user to change their attendance status.
	buttons := tgp.eventUIButtons(event, usrInfo.Locale)

	rich := tgbotapi.NewSendRichMessage(cb.Message.Chat.ID, tgp.makeEventUIRichMessage(event, usrInfo.Locale))
	rich.ReplyMarkup = &buttons

	_, err = tgp.tg.Send(rich)
	if !shouldFallBackToClassic(err) {
		if err != nil {
			log.Println(err)
		}
		return
	}
	log.Printf("Rich send of event %v failed, falling back to classic: %v\n", eventId, err)

	mObj := tgbotapi.NewMessage(cb.Message.Chat.ID, tgp.makeEventUIText(event, usrInfo.Locale))
	mObj.ReplyMarkup = &buttons
	mObj.ParseMode = ParseModeHtml
	mObj.LinkPreviewOptions.IsDisabled = true

	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}
