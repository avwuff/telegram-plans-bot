package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strconv"
	"strings"
)

func initUICommands(cmds *tgCommands.CommandList) {

	cmds.AddCB(tgCommands.Callback{DataPrefix: "use", Public: true, Handler: ui_Activate})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "attending", Public: true, Handler: ui_Attending})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "maybe", Public: true, Handler: ui_Attending})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "cancel", Public: true, Handler: ui_Attending})

}

func ui_Activate(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// note that usrInfo may represent a user who has never used the bot.

	// Update the message with info about the event.
	// Command is use:<event id>:activate
	data := strings.Split(cb.Data, ":")
	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}

	event, loc, err := dbHelper.GetEvent(uint(eventId), int64(cb.From.ID))
	if err != nil {
		return
	}
	if loc == nil {
		loc = usrInfo.Locale
	}

	makeEventUI(tg, int64(cb.From.ID), event, loc, cb.InlineMessageID)

}

func ui_Attending(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// note that usrInfo may represent a user who has never used the bot.
	data := strings.Split(cb.Data, ":")
	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}
	event, loc, err := dbHelper.GetEvent(uint(eventId), int64(cb.From.ID))
	if err != nil {
		return
	}
	// TODO: All events should pick up the creator's locale.
	if loc == nil {
		loc = usrInfo.Locale
	}

	// Update the attending data about the event.
	var reply dbHelper.AttendMsgs
	switch data[0] {
	case "attending":
		// How many people are they bringing?
		people, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		reply = dbHelper.Attending(event, int64(cb.From.ID), cb.From.FirstName, dbHelper.CANATTEND_YES, people)

	case "maybe":
		reply = dbHelper.Attending(event, int64(cb.From.ID), cb.From.FirstName, dbHelper.CANATTEND_MAYBE, 0)
	case "cancel":
		reply = dbHelper.Attending(event, int64(cb.From.ID), cb.From.FirstName, dbHelper.CANATTEND_NO, 0)
	}

	// Send the reply.
	txt := ""
	switch reply {
	case dbHelper.ATTEND_ADDED:
		txt = loc.Sprintf("Alright, you've been marked as attending.")
	case dbHelper.ATTEND_MAYBE:
		txt = loc.Sprintf("Alright, you've been marked as maybe.")
	case dbHelper.ATTEND_FULL:
		txt = loc.Sprintf("Sorry, this event is currently full!")
	case dbHelper.ATTEND_REMOVED:
		txt = loc.Sprintf("Alright, you've been marked as unable to attend.")
	default:
		txt = loc.Sprintf(GENERAL_ERROR)
	}
	answerCallback(tg, cb, txt)

	makeEventUI(tg, int64(cb.From.ID), event, loc, cb.InlineMessageID)
}

func answerCallback(tg *tgWrapper.Telegram, query *tgbotapi.CallbackQuery, Text string) {
	callbackConfg := tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            Text,
	}
	if _, err := tg.AnswerCallbackQuery(callbackConfg); err != nil {
		log.Println(err)
	}
}

// makeEventUI displays the main event UI.
func makeEventUI(tg *tgWrapper.Telegram, chatId int64, event *dbHelper.FurryPlans, loc *localizer.Localizer, inlineId string) {

	t := "TODO NOT YET FINISHED"
	t += "<b>" + loc.Sprintf("Name:") + "</b> " + event.Name + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDate(event.DateTime.Time) + "\n"
	t += "<b>" + loc.Sprintf("Location:") + "</b> " + event.Location + "\n"
	t += "<b>" + loc.Sprintf("Hosted By:") + "</b> " + event.OwnerName + "\n"

	var mObj tgbotapi.Chattable

	buttons := eventUIButtons(event, loc)

	mObj2 := tgbotapi.NewEditMessageText(chatId, 0, t)
	mObj2.InlineMessageID = inlineId
	mObj2.ParseMode = "HTML"
	mObj2.ReplyMarkup = &buttons
	mObj2.DisableWebPagePreview = true
	mObj = mObj2

	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// eventEditButtons creates the buttons that help you edit an event.
func eventUIButtons(event *dbHelper.FurryPlans, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton

	if event.Suitwalk == 1 {
		//row := make([]tgbotapi.InlineKeyboardButton, 0)
		// TODO
	} else {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️ I'm going!"), fmt.Sprintf("attending:%v:0", event.EventID)))
		row = append(row, quickButton(loc.Sprintf("👭 Me +1"), fmt.Sprintf("attending:%v:1", event.EventID)))
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️👭 Me +2"), fmt.Sprintf("attending:%v:2", event.EventID)))
		buttons = append(buttons, row)
	}

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	if event.DisableMaybe == 0 {
		row = append(row, quickButton(loc.Sprintf("🤔️ Maybe"), fmt.Sprintf("maybe:%v", event.EventID)))
	}
	row = append(row, quickButton(loc.Sprintf("❌️ I can't make it"), fmt.Sprintf("cancel:%v", event.EventID)))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	// TODO FINISH THIS BUTTON
	row = append(row, quickButton(loc.Sprintf("📆 Add to Calendar"), fmt.Sprintf("use:%v:activate", event.EventID)))
	buttons = append(buttons, row)

	if event.AllowShare == 1 {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		// TODO FINISH THIS ONE
		row = append(row, quickButton(loc.Sprintf("📩 Share to another chat..."), fmt.Sprintf("attending:%v:0", event.EventID)))
		buttons = append(buttons, row)
	}

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
