package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	event, err := db.GetEvent(uint(eventId), -1)

	if err != nil {
		return
	}
	loc := localizer.FromLanguage(event.Language())

	// Save where this was posted
	// We can use a Gofunc here since it isn't important to have this saved before we continue
	go event.SavePosting(cb.InlineMessageID)

	// HTML format the name so it works properly.
	name := tg.ConvertEntitiesToHTML(cb.From.FirstName, nil)

	// Update the attending data about the event.
	var reply dbInterface.AttendMsgs
	switch data[0] {
	case "use": // Event activated
		reply = dbInterface.ATTEND_ACTIVE
	case "attending":
		// How many people are they bringing?
		people, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}

		canAtt := dbInterface.CANATTEND_YES
		if people >= int(dbInterface.CANATTEND_PHOTOGRAPHER) {
			canAtt = dbInterface.CANATTEND_PHOTOGRAPHER
			people -= int(dbInterface.CANATTEND_PHOTOGRAPHER)
		}
		if people >= int(dbInterface.CANATTEND_SUITING) {
			canAtt = dbInterface.CANATTEND_SUITING
			people -= int(dbInterface.CANATTEND_SUITING)
		}

		reply = event.Attending(cb.From.ID, name, canAtt, people)

	case "maybe":
		reply = event.Attending(cb.From.ID, name, dbInterface.CANATTEND_MAYBE, 0)
	case "cancel":
		reply = event.Attending(cb.From.ID, name, dbInterface.CANATTEND_NO, 0)
	}

	// Send the reply.
	txt := ""
	switch reply {
	case dbInterface.ATTEND_ADDED:
		txt = loc.Sprintf("Alright, you've been marked as attending.")
	case dbInterface.ATTEND_MAYBE:
		txt = loc.Sprintf("Alright, you've been marked as maybe.")
	case dbInterface.ATTEND_FULL:
		txt = loc.Sprintf("Sorry, this event is currently full!")
		answerCallback(tg, cb, txt)
		return
	case dbInterface.ATTEND_REMOVED:
		txt = loc.Sprintf("Alright, you've been marked as unable to attend.")
	case dbInterface.ATTEND_ACTIVE:
		txt = loc.Sprintf("Event is ready to be used!")
	default:
		txt = loc.Sprintf("A general error occurred.") // Can't use the CONST here because it crashes GOTEXT.
		answerCallback(tg, cb, txt)
		return
	}
	// Answer the callback in a Gofunc
	go answerCallback(tg, cb, txt)

	// Also update the event in all the places
	updateEventUIAllPostings(tg, event)
}

// Every time the event UI needs to be updated, do it in all the places.
func updateEventUIAllPostings(tg *tgWrapper.Telegram, event dbInterface.DBEvent) {

	// Use the localizer from the event.
	loc := localizer.FromLanguage(event.Language())

	postings, err := event.Postings()
	if err != nil {
		return
	}

	for i := range postings {
		// We do this as a Gofunc so that they can all be updated at once.
		go func(msgId string) {

			// Make sure this one isn't in the Retry After queue.
			if !inQueue(msgId) {
				// Update this one.
				retryAfter, err := makeEventUI(tg, 0, event, loc, msgId)
				if err != nil {
					// Was this a "too many requests" message?
					if strings.Contains(err.Error(), "Too Many Requests") {
						// Retry this one after this time.
						addToQueue(tg, event, msgId, retryAfter)
					}

					if strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
						// The message where this once was, was probably deleted.
						// So we delete the posting, so we don't try it again.
						event.DeletePosting(msgId)
					}
				}
			}
		}(postings[i])
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
func makeEventUI(tg *tgWrapper.Telegram, chatId int64, event dbInterface.DBEvent, loc *localizer.Localizer, inlineId string) (int, error) {

	URL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%v", url.QueryEscape(helpers.StripHtmlRegex(event.Location())))

	t := "<b>" + event.Name() + "</b> " + loc.Sprintf("hosted by") + " " + event.OwnerName() + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDateForLocale(event.DateTime()) + "\n"
	t += "<b>" + loc.Sprintf("Location:") + "</b> <a href=\"" + URL + "\">" + event.Location() + "</a>" + "\n"
	if event.MaxAttendees() > 0 {
		t += "<b>" + loc.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees()) + "\n"
	}
	if event.Notes() != "" {
		t += "<b>" + loc.Sprintf("Notes:") + "</b>\n" + event.Notes() + "\n"
	}

	// Get the list of people attending
	attending, err := event.GetAttending()

	var cGoing, cSuiting, cPhoto int
	var tGoing, tMaybe, tNot []string
	var tSuiting, tPhoto []string

	if err != nil {
		t += "Unable to get list of attending people."
	} else {

		for _, attend := range attending {

			switch dbInterface.CanAttend(attend.CanAttend) {
			case dbInterface.CANATTEND_YES: // Going / Spotting

				txt := fmt.Sprintf(` - <a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				if attend.PlusMany > 0 {
					txt += fmt.Sprintf(" <b>+%v</b>", attend.PlusMany)
				}
				tGoing = append(tGoing, txt)
				cGoing += 1 + attend.PlusMany

			case dbInterface.CANATTEND_SUITING:

				txt := fmt.Sprintf(` - <a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				if attend.PlusMany > 0 {
					txt += fmt.Sprintf(" <b>+%v</b>", attend.PlusMany)
				}
				tSuiting = append(tSuiting, txt)
				cSuiting += 1 + attend.PlusMany

			case dbInterface.CANATTEND_PHOTOGRAPHER:

				txt := fmt.Sprintf(` - <a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				if attend.PlusMany > 0 {
					txt += fmt.Sprintf(" <b>+%v</b>", attend.PlusMany)
				}
				tPhoto = append(tPhoto, txt)
				cPhoto += 1 + attend.PlusMany

			case dbInterface.CANATTEND_MAYBE: // Maybe

				txt := fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				tMaybe = append(tMaybe, txt)
			default: // Not going

				txt := fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)
				tNot = append(tNot, txt)
			}
		}
	}

	// Fursuit walks display different event messages
	if event.Suitwalk() {
		if len(tSuiting) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Suiting: %v", cSuiting) + "</b>\n"
			t += strings.Join(tSuiting, "\n") + "\n"
		}
		if len(tPhoto) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Photographers: %v", cPhoto) + "</b>\n"
			t += strings.Join(tPhoto, "\n") + "\n"
		}
		if len(tGoing) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Spotting: %v", cGoing) + "</b>\n"
			t += strings.Join(tGoing, "\n") + "\n"
		}

	} else {
		if len(tGoing) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Attending: %v", cGoing) + "</b>\n"
			t += strings.Join(tGoing, "\n") + "\n"
		}
	}

	if len(tMaybe) > 0 {
		t += "\n" + "<b>" + loc.Sprintf("Maybe: %v", len(tMaybe)) + "</b>\n"
		t += strings.Join(tMaybe, ", ") + "\n"
	}

	if len(tNot) > 0 {
		t += "\n" + "<b>" + loc.Sprintf("Can't make it: %v", len(tNot)) + "</b>\n"
		t += strings.Join(tNot, ", ") + "\n"
	}

	t += "\n<i>" + loc.Sprintf("Can you go? Use the buttons below.") + "</i>"

	var mObj tgbotapi.Chattable

	buttons := eventUIButtons(event, loc)

	mObj2 := tgbotapi.NewEditMessageText(chatId, 0, t)
	mObj2.InlineMessageID = inlineId
	mObj2.ParseMode = tgWrapper.ParseModeHtml
	mObj2.ReplyMarkup = &buttons
	mObj2.DisableWebPagePreview = true
	mObj = mObj2

	rsp, err := tg.Request(mObj)
	if err != nil {
		if rsp.Parameters != nil {
			return rsp.Parameters.RetryAfter, err
		}
		return 0, err
	}
	return 0, nil
}

// eventEditButtons creates the buttons that help you edit an event.
func eventUIButtons(event dbInterface.DBEvent, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton

	if event.Suitwalk() {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("🐕‍🦺 I'm Suiting"), fmt.Sprintf("attending:%v:20", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🐕‍🦺🐱 Suiting +1"), fmt.Sprintf("attending:%v:21", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🐕‍🦺🐾 Suiting +2"), fmt.Sprintf("attending:%v:22", event.ID())))
		buttons = append(buttons, row)
		row = make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("📷 Photographer"), fmt.Sprintf("attending:%v:30", event.ID())))
		row = append(row, quickButton(loc.Sprintf("📷🎥 Photo +1"), fmt.Sprintf("attending:%v:31", event.ID())))
		row = append(row, quickButton(loc.Sprintf("📷🎞 Photo +2"), fmt.Sprintf("attending:%v:32", event.ID())))
		buttons = append(buttons, row)
		row = make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️ Spotting"), fmt.Sprintf("attending:%v:0", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️🕺 Spotting +1"), fmt.Sprintf("attending:%v:1", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️👭 Spotting +2"), fmt.Sprintf("attending:%v:2", event.ID())))
		buttons = append(buttons, row)
	} else {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️ I'm going!"), fmt.Sprintf("attending:%v:0", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️🕺 Me +1"), fmt.Sprintf("attending:%v:1", event.ID())))
		row = append(row, quickButton(loc.Sprintf("🙋‍♂️👭 Me +2"), fmt.Sprintf("attending:%v:2", event.ID())))
		buttons = append(buttons, row)
	}

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	if !event.DisableMaybe() {
		row = append(row, quickButton(loc.Sprintf("🤔️ Maybe"), fmt.Sprintf("maybe:%v", event.ID())))
	}
	row = append(row, quickButton(loc.Sprintf("❌️ I can't make it"), fmt.Sprintf("cancel:%v", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	// TODO: This URL needs to be configurable
	addUrl := fmt.Sprintf("https://plansbot.avbrand.com/add/%v.html", helpers.CalenFeedMD5(saltValue, int64(event.ID())))
	row = append(row, tgbotapi.InlineKeyboardButton{
		Text: loc.Sprintf("📆 Add to Calendar"),
		URL:  &addUrl,
	})

	buttons = append(buttons, row)

	if event.SharingAllowed() {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		shareButton := fmt.Sprintf("%v%v", SHARE_PREFIX, helpers.CalenFeedMD5(saltValue, int64(event.ID()))) // Example: POST:1234
		row = append(row, tgbotapi.InlineKeyboardButton{
			Text:              loc.Sprintf("📩 Share to another chat..."),
			SwitchInlineQuery: &shareButton,
		})
		buttons = append(buttons, row)
	}

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
