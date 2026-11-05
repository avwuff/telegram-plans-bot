package tgPlansBot

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
)

func (tgp *TGPlansBot) initUICommands() {

	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "use", Public: true, Handler: tgp.ui_Attending})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "nothing", Public: true, Handler: tgp.ui_Attending})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "attending", Public: true, Handler: tgp.ui_Attending})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "maybe", Public: true, Handler: tgp.ui_Attending})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "cancel", Public: true, Handler: tgp.ui_Attending})

}

func (tgp *TGPlansBot) ui_Attending(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// note that usrInfo may represent a user who has never used the bot.
	data := strings.Split(cb.Data, ":")
	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}
	event, err := tgp.db.GetEvent(uint(eventId), -1)

	if err != nil {
		return
	}
	loc := localizer.FromLanguage(event.Language())

	// Save where this was posted
	// We can use a Gofunc here since it isn't important to have this saved before we continue
	if cb.InlineMessageID != "" {
		go func() {
			err := event.SavePosting(cb.InlineMessageID)
			if err != nil {
				log.Println(err)
			}
		}()
	} else {
		go func() {
			err := event.SavePostingRegular(cb.Message.Chat.ID, cb.Message.MessageID)
			if err != nil {
				log.Println(err)
			}
		}()
	}

	// HTML format the name so it works properly.
	name := helpers.HtmlEntities(cb.From.FirstName)

	var reply dbInterface.AttendMsgs

	// If the event is closed, then don't let the user change anything.
	if event.Closed() {
		data[0] = "nothing"
	}

	// Update the attending data about the event.
	switch data[0] {
	case "use": // Event activated
		reply = dbInterface.ATTEND_ACTIVE
	case "nothing": // Event activated
		reply = dbInterface.ATTEND_CLOSED
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

		// If we had previously specified the names of our guests, don't erase that information.
		attend, _ := event.GetAttending(cb.From.ID)
		var guests []string
		if len(attend) > 0 {
			guests = attend[0].Guests
			// strip the number of guests to how many guests we are bringing.
			if people < len(guests) {
				guests = guests[0:people]
			}
		}

		reply = event.Attending(cb.From.ID, name, canAtt, people, guests)

	case "maybe":
		reply = event.Attending(cb.From.ID, name, dbInterface.CANATTEND_MAYBE, 0, nil)
	case "cancel":
		reply = event.Attending(cb.From.ID, name, dbInterface.CANATTEND_NO, 0, nil)
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
		tgp.answerCallback(cb, txt)
		return
	case dbInterface.ATTEND_REMOVED:
		txt = loc.Sprintf("Alright, you've been marked as unable to attend.")
	case dbInterface.ATTEND_ACTIVE:
		txt = loc.Sprintf("Event is ready to be used!")
	case dbInterface.ATTEND_CLOSED:
		txt = loc.Sprintf("Event is closed, please contact the host for more information.")
		tgp.answerCallback(cb, txt)
		return
	default:
		txt = loc.Sprintf("A general error occurred.") // Can't use the CONST here because it crashes GOTEXT.
		tgp.answerCallback(cb, txt)
		return
	}
	// Answer the callback in a Gofunc
	go tgp.answerCallback(cb, txt)

	if cb.InlineMessageID != "" {
		tgp.updateEventPosting(event, cb.InlineMessageID)
	} else {
		tgp.updateEventPostingRegular(event, cb.Message.Chat.ID, cb.Message.MessageID)
	}

	// Also update the event in all the places
	tgp.updateEventUIAllPostings(event)
}

// Every time the event UI needs to be updated, do it in all the places.
func (tgp *TGPlansBot) updateEventUIAllPostings(event dbInterface.DBEvent) {

	postings, err := event.Postings()
	if err != nil {
		return
	}

	for i := range postings {
		// We do this as a Gofunc so that they can all be updated at once.
		go func(posting dbInterface.Posting) {

			// Make sure this one isn't in the Retry After queue.
			if posting.InlineMessageID != "" {
				if !tgp.inQueue(posting.InlineMessageID) {
					tgp.updateEventPosting(event, posting.InlineMessageID)
				}
			} else {
				if !tgp.inQueueRegular(posting.ChatID, posting.MessageId) {
					tgp.updateEventPostingRegular(event, posting.ChatID, posting.MessageId)
				}
			}

		}(postings[i])
	}

}

func (tgp *TGPlansBot) updateEventPosting(event dbInterface.DBEvent, msgId string) {
	// Use the localizer from the event.
	loc := localizer.FromLanguage(event.Language())

	// Update this one.
	retryAfter, err := tgp.makeEventUI(0, event, loc, msgId)
	if err != nil {
		// Was this a "too many requests" message?
		if strings.Contains(err.Error(), "Too Many Requests") {
			// Retry this one after this time.
			tgp.addToQueue(event, msgId, retryAfter)
		}

		if strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
			// The message where this once was, was probably deleted.
			// So we delete the posting, so we don't try it again.
			event.DeletePosting(msgId)
		}
	}
}

// updateEventPostingRegular is the same as updateEventPosting, but it works on non-inline messages
func (tgp *TGPlansBot) updateEventPostingRegular(event dbInterface.DBEvent, chatId int64, messageId int) {
	// Use the localizer from the event.
	loc := localizer.FromLanguage(event.Language())

	// Update this one.
	retryAfter, err := tgp.makeEventUIRegular(chatId, event, loc, messageId)
	if err != nil {
		// Was this a "too many requests" message?
		if strings.Contains(err.Error(), "Too Many Requests") {
			// Retry this one after this time.
			tgp.addToQueueRegular(event, chatId, messageId, retryAfter)
		}

		if strings.Contains(err.Error(), "message to edit not found") {
			// The message where this once was, was probably deleted.
			// So we delete the posting, so we don't try it again.
			event.DeletePostingRegular(chatId, messageId)
		}
	}
}

func (tgp *TGPlansBot) answerCallback(query *tgbotapi.CallbackQuery, Text string) {
	callbackConfg := tgbotapi.CallbackConfig{
		CallbackQueryID: query.ID,
		Text:            Text,
	}
	if _, err := tgp.tg.AnswerCallbackQuery(callbackConfg); err != nil {
		log.Println(err)
	}
}

// eventPictureMediaID is the name used to refer to the event picture inside a rich message.
const eventPictureMediaID = "eventpicture"

// collapseListsOver is how many people a list of people can have before a rich message collapses it.
const collapseListsOver = 50

// makeEventUIText creates the event text for a classic message.
func (tgp *TGPlansBot) makeEventUIText(event dbInterface.DBEvent, loc *localizer.Localizer) string {
	return tgp.buildEventUIText(event, loc, false)
}

// makeEventUIRichHtml creates the event text for a rich message.
func (tgp *TGPlansBot) makeEventUIRichHtml(event dbInterface.DBEvent, loc *localizer.Localizer) string {
	return helpers.TelegramHtmlToRichHtml(tgp.buildEventUIText(event, loc, true))
}

// makeEventUIRichMessage creates the rich message for an event, with the picture at the top if it has one.
func (tgp *TGPlansBot) makeEventUIRichMessage(event dbInterface.DBEvent, loc *localizer.Localizer) tgbotapi.InputRichMessage {
	rich := tgbotapi.InputRichMessage{HTML: tgp.makeEventUIRichHtml(event, loc)}

	if fileID := event.PictureFileID(); fileID != "" {
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileID(fileID))
		rich.HTML = `<img src="tg://photo?id=` + eventPictureMediaID + `"/>` + rich.HTML
		rich.Media = []tgbotapi.InputRichMessageMedia{{ID: eventPictureMediaID, Media: &photo}}
	}
	return rich
}

// buildEventUIText creates the event text in classic Telegram HTML.
// If rich is set, it can also include details blocks, which only rich messages support.
func (tgp *TGPlansBot) buildEventUIText(event dbInterface.DBEvent, loc *localizer.Localizer, rich bool) string {

	URL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%v", url.QueryEscape(helpers.StripHtmlRegex(event.Location())))

	t := "<b>" + event.Name() + "</b> " + loc.Sprintf("hosted by") + " " + event.OwnerName() + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDateAndEndDateForLocale(event.DateTime(), event.EndDateTime(), "<b>"+loc.Sprintf("Ends:")+"</b> ") + "\n"
	t += "<b>" + loc.Sprintf("Location:") + "</b> <a href=\"" + URL + "\">" + event.Location() + "</a>" + "\n"
	if event.MaxAttendees() > 0 {
		t += "<b>" + loc.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees()) + "\n"
	}
	if event.TotalCost() > 0 {
		donationsSoFar, _ := event.GetDonationTotal()

		t += loc.Sprintf("💰 <b>Help cover the cost of this event!</b> We're at <b>%.2f</b> of <b>%d</b>. Use the Donate button below to donate.", donationsSoFar, event.TotalCost()) + "\n"
	}
	if event.Notes() != "" {
		t += "<b>" + loc.Sprintf("Notes:") + "</b>\n" + event.Notes() + "\n"
	}

	// Get the list of people attending
	attending, err := event.GetAttending(-1)

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

				// Do we have named guests?
				for _, guest := range attend.Guests {
					txtG := fmt.Sprintf(`   - <i>%v</i>`, guest)
					tGoing = append(tGoing, txtG)
				}

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

	attendingCollapsed := false

	// Fursuit walks display different event messages
	if event.Suitwalk() {
		if len(tSuiting) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Suiting: %v", cSuiting) + "</b>\n"
			if !event.HideNames() {
				t += strings.Join(tSuiting, "\n") + "\n"
			}
		}
		if len(tPhoto) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Photographers: %v", cPhoto) + "</b>\n"
			if !event.HideNames() {
				t += strings.Join(tPhoto, "\n") + "\n"
			}
		}
		if len(tGoing) > 0 {
			t += "\n" + "<b>" + loc.Sprintf("Spotting: %v", cGoing) + "</b>\n"
			if !event.HideNames() {
				t += strings.Join(tGoing, "\n") + "\n"
			}
		}

	} else {
		attendingCollapsed = rich && cGoing > collapseListsOver
		if len(tGoing) > 0 {
			t += eventUIListSection(loc.Sprintf("Attending: %v", cGoing), tGoing, "\n", event.HideNames(), attendingCollapsed)
		}
	}

	// Maybe and Can't make it are never left open when Attending is collapsed.
	if len(tMaybe) > 0 {
		collapsed := rich && (attendingCollapsed || len(tMaybe) > collapseListsOver)
		t += eventUIListSection(loc.Sprintf("Maybe: %v", len(tMaybe)), tMaybe, ", ", event.HideNames(), collapsed)
	}

	if len(tNot) > 0 {
		collapsed := rich && (attendingCollapsed || len(tNot) > collapseListsOver)
		t += eventUIListSection(loc.Sprintf("Can't make it: %v", len(tNot)), tNot, ", ", event.HideNames(), collapsed)
	}

	t += "\n<i>" + loc.Sprintf("Can you go? Use the buttons below.") + "</i>"
	return t
}

// eventUIListSection creates one of the lists of people, with a heading.
// A collapsed list is a details block, which only rich messages support. It has no 'open' attribute, so it starts closed.
func eventUIListSection(heading string, names []string, separator string, hideNames bool, collapsed bool) string {
	heading = "<b>" + heading + "</b>"
	if hideNames {
		return "\n" + heading + "\n"
	}
	list := strings.Join(names, separator)
	if collapsed {
		return "\n<details><summary>" + heading + "</summary>" + helpers.TelegramHtmlToRichHtml(list) + "</details>\n"
	}
	return "\n" + heading + "\n" + list + "\n"
}

// makeEventUI displays the main event UI.
func (tgp *TGPlansBot) makeEventUI(chatId int64, event dbInterface.DBEvent, loc *localizer.Localizer, inlineId string) (int, error) {

	buttons := tgp.eventUIButtons(event, loc)

	rich := tgbotapi.NewEditMessageText(chatId, 0, "")
	rich.InlineMessageID = inlineId
	rich.RichMessage = tgp.makeEventUIRichMessage(event, loc)
	rich.ReplyMarkup = &buttons

	retryAfter, err := tgp.requestEventEdit(rich)
	if !shouldFallBackToClassic(err) {
		return retryAfter, err
	}
	log.Printf("Rich edit of inline message %v failed, falling back to classic: %v\n", inlineId, err)

	mObj := tgbotapi.NewEditMessageText(chatId, 0, tgp.makeEventUIText(event, loc))
	mObj.InlineMessageID = inlineId
	mObj.ParseMode = ParseModeHtml
	mObj.ReplyMarkup = &buttons
	mObj.LinkPreviewOptions.IsDisabled = true

	return tgp.requestEventEdit(mObj)
}

// makeEventUIRegular displays the main event UI for non-inline messages
func (tgp *TGPlansBot) makeEventUIRegular(chatId int64, event dbInterface.DBEvent, loc *localizer.Localizer, messageId int) (int, error) {

	buttons := tgp.eventUIButtons(event, loc)

	rich := tgbotapi.NewEditMessageText(chatId, messageId, "")
	rich.RichMessage = tgp.makeEventUIRichMessage(event, loc)
	rich.ReplyMarkup = &buttons

	retryAfter, err := tgp.requestEventEdit(rich)
	if !shouldFallBackToClassic(err) {
		return retryAfter, err
	}
	log.Printf("Rich edit of message %v in chat %v failed, falling back to classic: %v\n", messageId, chatId, err)

	mObj := tgbotapi.NewEditMessageText(chatId, messageId, tgp.makeEventUIText(event, loc))
	mObj.ParseMode = ParseModeHtml
	mObj.ReplyMarkup = &buttons
	mObj.LinkPreviewOptions.IsDisabled = true

	return tgp.requestEventEdit(mObj)
}

// requestEventEdit sends an edit to Telegram, and returns how long to wait before retrying if we were rate limited.
func (tgp *TGPlansBot) requestEventEdit(edit tgbotapi.Chattable) (int, error) {
	rsp, err := tgp.tg.Request(edit)
	if err != nil {
		// The response is nil if the request never got an answer from Telegram.
		if rsp != nil && rsp.Parameters != nil {
			return rsp.Parameters.RetryAfter, err
		}
		return 0, err
	}
	return 0, nil
}

// shouldFallBackToClassic decides if a failed rich edit is worth retrying as a classic message.
// Telegram could reject the rich content itself, but these errors would apply to a classic edit too.
func shouldFallBackToClassic(err error) bool {
	if err == nil {
		return false
	}
	for _, msg := range []string{"Too Many Requests", "MESSAGE_ID_INVALID", "message to edit not found", "message is not modified"} {
		if strings.Contains(err.Error(), msg) {
			return false
		}
	}
	return true
}

// eventEditButtons creates the buttons that help you edit an event.
func (tgp *TGPlansBot) eventUIButtons(event dbInterface.DBEvent, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton

	// If the event is closed, remove all the buttons!
	if event.Closed() {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("❌️ Event Closed"), fmt.Sprintf("nothing:%v", event.ID())))
		buttons = append(buttons, row)

		return tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		}
	}

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

		emoji1 := "🙋‍♂️🕺 "
		emoji2 := "🙋‍♂️👭 "
		emoji3 := "🙋‍♂️👨‍👩‍👦 "

		if event.MaxGuests() >= 1 {

			// if it is just one guest...
			if event.MaxGuests() == 1 {
				row = append(row, quickButton(emoji1+loc.Sprintf("Me +1"), fmt.Sprintf("attending:%v:1", event.ID())))
			}

			// Create the text for the button for inline, to get the guest ID sharing UI
			specGuestText := fmt.Sprintf("%v%v", GUESTS_PREFIX, helpers.CalenFeedMD5(tgp.saltValue+GUEST_HASH_EXTRA, int64(event.ID())))
			row = append(row, tgbotapi.InlineKeyboardButton{
				Text:                         loc.Sprintf("Guest Names..."),
				SwitchInlineQueryCurrentChat: &specGuestText,
			})
		}
		buttons = append(buttons, row)

		// Only show the +1 buttons if guests are configured
		if event.MaxGuests() >= 2 {
			row = make([]tgbotapi.InlineKeyboardButton, 0)

			if event.MaxGuests() >= 1 {
				row = append(row, quickButton(emoji1+loc.Sprintf("Me +1"), fmt.Sprintf("attending:%v:1", event.ID())))
			}
			if event.MaxGuests() >= 2 {
				row = append(row, quickButton(emoji2+loc.Sprintf("+2"), fmt.Sprintf("attending:%v:2", event.ID())))
			}
			if event.MaxGuests() >= 3 {
				row = append(row, quickButton(emoji3+loc.Sprintf("+3"), fmt.Sprintf("attending:%v:3", event.ID())))
			}
			buttons = append(buttons, row)
		}
	}

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	if !event.DisableMaybe() {
		row = append(row, quickButton(loc.Sprintf("🤔️ Maybe"), fmt.Sprintf("maybe:%v", event.ID())))
	}
	row = append(row, quickButton(loc.Sprintf("❌️ I can't make it"), fmt.Sprintf("cancel:%v", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	// TODO: This URL needs to be configurable
	addUrl := fmt.Sprintf("https://plansbot.avbrand.com/add/%v.html", helpers.CalenFeedMD5(tgp.saltValue, int64(event.ID())))
	row = append(row, tgbotapi.InlineKeyboardButton{
		Text: loc.Sprintf("📆 Add to Calendar"),
		URL:  &addUrl,
	})

	buttons = append(buttons, row)

	if event.TotalCost() > 0 {

		row = make([]tgbotapi.InlineKeyboardButton, 0)

		// Create the text for the button for inline, to get the guest ID sharing UI
		specDonateText := fmt.Sprintf("%v%v", DONATE_PREFIX, helpers.CalenFeedMD5(tgp.saltValue+GUEST_HASH_EXTRA, int64(event.ID())))
		row = append(row, tgbotapi.InlineKeyboardButton{
			Text:                         loc.Sprintf("💰 Donate! Help Cover Costs!"),
			SwitchInlineQueryCurrentChat: &specDonateText,
		})

		buttons = append(buttons, row)
	}

	if event.SharingAllowed() {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		shareButton := fmt.Sprintf("%v%v", SHARE_PREFIX, helpers.CalenFeedMD5(tgp.saltValue, int64(event.ID()))) // Example: POST:1234
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
