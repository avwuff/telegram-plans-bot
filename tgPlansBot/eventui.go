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
	"net/url"
	"strconv"
	"strings"
)

func initUICommands(cmds *tgCommands.CommandList) {

	cmds.AddCB(tgCommands.Callback{DataPrefix: "use", Public: true, Handler: ui_Attending})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "attending", Public: true, Handler: ui_Attending})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "maybe", Public: true, Handler: ui_Attending})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "cancel", Public: true, Handler: ui_Attending})

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

	// Save where this was posted
	// We can use a Gofunc here since it isn't important to have this saved before we continue
	go event.SavePosting(cb.InlineMessageID)

	// HTML format the name so it works properly.
	name := tg.ConvertEntitiesToHTML(cb.From.FirstName, nil)

	// Update the attending data about the event.
	var reply dbHelper.AttendMsgs
	switch data[0] {
	case "use": // Event activated
		reply = dbHelper.ATTEND_ACTIVE
	case "attending":
		// How many people are they bringing?
		people, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		reply = event.Attending(int64(cb.From.ID), name, dbHelper.CANATTEND_YES, people)

	case "maybe":
		reply = event.Attending(int64(cb.From.ID), name, dbHelper.CANATTEND_MAYBE, 0)
	case "cancel":
		reply = event.Attending(int64(cb.From.ID), name, dbHelper.CANATTEND_NO, 0)
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
	case dbHelper.ATTEND_ACTIVE:
		txt = loc.Sprintf("Event is ready to be used!")
	default:
		txt = loc.Sprintf("A general error occurred.") // Can't use the CONST here because it crashes GOTEXT.
	}
	// Answer the callback in a Gofunc
	go answerCallback(tg, cb, txt)
	err = makeEventUI(tg, int64(cb.From.ID), event, loc, cb.InlineMessageID)
	if err != nil {
		log.Println(err)
	}

	// Also update the event in all the places
	updateEventUIAllPostings(tg, event, cb.InlineMessageID)
}

// Every time the event UI needs to be updated, do it in all the places.
func updateEventUIAllPostings(tg *tgWrapper.Telegram, event *dbHelper.FurryPlans, skipPosting string) {

	// Use the localizer from the event.
	loc := localizer.FromLanguage(event.Language)

	postings, err := event.Postings()
	if err != nil {
		return
	}

	for i := range postings {
		if postings[i].MessageID != skipPosting {
			// We do this as a Gofunc so that they can all be updated at once.
			go func(msgId string) {
				// Update this one.
				err := makeEventUI(tg, 0, event, loc, msgId)
				if err != nil {
					//log.Println("Failed on ID", msgId, err)
					if strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
						// The message where this once was, was probably deleted.
						// So we delete the posting, so we don't try it again.
						event.DeletePosting(msgId)
					}
				}
			}(postings[i].MessageID)
		}
	}

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
func makeEventUI(tg *tgWrapper.Telegram, chatId int64, event *dbHelper.FurryPlans, loc *localizer.Localizer, inlineId string) error {

	URL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%v", url.QueryEscape(stripHtmlRegex(event.Location)))

	// TODO: Localization
	t := "<b>" + event.Name + "</b> " + loc.Sprintf("hosted by") + " " + event.OwnerName + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDate(event.DateTime.Time) + "\n"
	t += "<b>" + loc.Sprintf("Location:") + "</b> <a href=\"" + URL + "\">" + event.Location + "</a>" + "\n"
	if event.MaxAttendees > 0 {
		t += "<b>" + loc.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees) + "\n"
	}
	if event.Notes != "" {
		t += "<b>Notes:</b>\n" + event.Notes + "\n"
	}

	// Get the list of people attending
	attending, err := event.GetAttending()

	var cGoing int
	var tGoing, tMaybe, tNot []string

	if err != nil {
		t += "Unable to get list of attending people."
	} else {

		for _, attend := range attending {

			switch dbHelper.CanAttend(attend.CanAttend) {
			case dbHelper.CANATTEND_YES: // Going / Spotting

				txt := fmt.Sprintf(` - <a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				if attend.PlusMany > 0 {
					txt += fmt.Sprintf(" <b>+%v</b>", attend.PlusMany)
				}
				tGoing = append(tGoing, txt)
				cGoing += 1 + attend.PlusMany

			/*case 20 ' Suiting
				tSuiting = tSuiting & " - " & "<a href="
				"tg://user?id=" & RS("userID") & ""
				">" & RS("UserName") & "</a>"
				if clng(RS("plusMany")) > 0 then
				tSuiting = tSuiting & " <b>+" & clng(RS("plusMany")) & "</b>"
				tSuiting = tSuiting & vbcrlf

				cSuiting = cSuiting + 1 + clng(RS("plusMany"))

			case 30 ' Photo
				tPhoto = tPhoto & " - " & "<a href="
				"tg://user?id=" & RS("userID") & ""
				">" & RS("UserName") & "</a>"
				if clng(RS("plusMany")) > 0 then
				tPhoto = tPhoto & " <b>+" & clng(RS("plusMany")) & "</b>"
				tPhoto = tPhoto & vbcrlf

				cPhoto = cPhoto + 1 + clng(RS("plusMany"))*/

			case 2: // Maybe

				txt := fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				tMaybe = append(tMaybe, txt)
			default: // Not going

				txt := fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				tNot = append(tNot, txt)
			}
		}
	}

	if event.Suitwalk == 1 {
		// TODO

	} else {
		if len(tGoing) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Attending: %v", cGoing) + "</b>\n"
			t += strings.Join(tGoing, "\n")
		}
	}

	if len(tMaybe) > 0 {
		t += "\n" + "<b>" + loc.Sprintf("Maybe: %v", len(tMaybe)) + "</b>\n"
		t += strings.Join(tMaybe, ", ")
	}

	if len(tNot) > 0 {
		t += "\n" + "<b>" + loc.Sprintf("Can't make it: %v", len(tNot)) + "</b>\n"
		t += strings.Join(tNot, ", ")
	}

	t += "\n<i>" + loc.Sprintf("Can you go? Use the buttons below.") + "</i>"

	var mObj tgbotapi.Chattable

	buttons := eventUIButtons(event, loc)

	mObj2 := tgbotapi.NewEditMessageText(chatId, 0, t)
	mObj2.InlineMessageID = inlineId
	mObj2.ParseMode = "HTML"
	mObj2.ReplyMarkup = &buttons
	mObj2.DisableWebPagePreview = true
	mObj = mObj2

	_, err = tg.Send(mObj)
	if err != nil {
		return err
	}
	return nil
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
