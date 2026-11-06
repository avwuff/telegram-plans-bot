package tgPlansBot

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

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

// richListRowsLimit is how many rows the lists of people can have in total before a rich message shows them as plain lines.
// Telegram allows 500 blocks per message, counting each list item and anything nested inside it.
const richListRowsLimit = 200

// eventUIPerson is someone who is coming to an event, along with anyone they're bringing.
type eventUIPerson struct {
	link     string
	plusMany int
	guests   []string
}

// label creates the text for this person, without their guests.
func (p eventUIPerson) label() string {
	if p.plusMany > 0 {
		return fmt.Sprintf("%v <b>+%v</b>", p.link, p.plusMany)
	}
	return p.link
}

// eventUIResponses is everyone who has responded to an event.
type eventUIResponses struct {
	going, suiting, photo    []eventUIPerson
	cGoing, cSuiting, cPhoto int
	maybe, not               []string
	err                      error
}

// coming is the number of people taking up a spot at the event, which is how the maximum is enforced.
func (p eventUIResponses) coming() int {
	return p.cGoing + p.cSuiting + p.cPhoto
}

// rows is how many lines the lists of people coming to the event would take up.
func (p eventUIResponses) rows() int {
	rows := 0
	for _, list := range [][]eventUIPerson{p.going, p.suiting, p.photo} {
		for _, person := range list {
			rows += 1 + len(person.guests)
		}
	}
	return rows
}

// gatherEventUIPeople sorts the people responding to an event by their status.
func gatherEventUIPeople(event dbInterface.DBEvent) eventUIResponses {
	var people eventUIResponses

	attending, err := event.GetAttending(-1)
	if err != nil {
		people.err = err
		return people
	}

	for _, attend := range attending {
		link := fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`, attend.UserID, attend.UserName)

		switch dbInterface.CanAttend(attend.CanAttend) {
		case dbInterface.CANATTEND_YES: // Going / Spotting
			people.going = append(people.going, eventUIPerson{link: link, plusMany: attend.PlusMany, guests: attend.Guests})
			people.cGoing += 1 + attend.PlusMany
		case dbInterface.CANATTEND_SUITING:
			people.suiting = append(people.suiting, eventUIPerson{link: link, plusMany: attend.PlusMany})
			people.cSuiting += 1 + attend.PlusMany
		case dbInterface.CANATTEND_PHOTOGRAPHER:
			people.photo = append(people.photo, eventUIPerson{link: link, plusMany: attend.PlusMany})
			people.cPhoto += 1 + attend.PlusMany
		case dbInterface.CANATTEND_MAYBE:
			people.maybe = append(people.maybe, link)
		default: // Not going
			people.not = append(people.not, link)
		}
	}
	return people
}

// eventIsFull checks if there are no spots left at an event.
func eventIsFull(event dbInterface.DBEvent, people eventUIResponses) bool {
	return event.MaxAttendees() > 0 && people.err == nil && people.coming() >= event.MaxAttendees()
}

// eventIsFunded checks if an event asking for help with its costs has reached its total.
func eventIsFunded(event dbInterface.DBEvent) bool {
	donationsSoFar, err := event.GetDonationTotal()
	return err == nil && donationsSoFar >= float64(event.TotalCost())
}

// eventUIMapURL creates the link used to look up the location of an event.
func eventUIMapURL(event dbInterface.DBEvent) string {
	return fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%v", url.QueryEscape(helpers.StripHtmlRegex(event.Location())))
}

// eventUICostText creates the message about the cost of an event. It's empty if the event isn't asking for help with its costs.
func eventUICostText(event dbInterface.DBEvent, loc *localizer.Localizer) string {
	if event.TotalCost() <= 0 {
		return ""
	}
	if eventIsFunded(event) {
		return loc.Sprintf("💰 <b>Fully funded, thank you!</b>")
	}
	donationsSoFar, _ := event.GetDonationTotal()
	return loc.Sprintf("💰 <b>Help cover the cost of this event!</b> We're at <b>%.2f</b> of <b>%d</b>. Use the Donate button below to donate.", donationsSoFar, event.TotalCost())
}

// eventUIStatusText creates the message shown when people can no longer respond. It's empty if they still can.
func eventUIStatusText(event dbInterface.DBEvent, people eventUIResponses, loc *localizer.Localizer) string {
	if event.Closed() {
		return loc.Sprintf("This event is closed.")
	}
	if eventIsFull(event, people) {
		return loc.Sprintf("This event is full.")
	}
	return ""
}

// makeEventUIText creates the event text for a classic message, in classic Telegram HTML.
func (tgp *TGPlansBot) makeEventUIText(event dbInterface.DBEvent, loc *localizer.Localizer) string {
	t := "<b>" + event.Name() + "</b> " + loc.Sprintf("hosted by") + " " + event.OwnerName() + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDateAndEndDateForLocale(event.DateTime(), event.EndDateTime(), "<b>"+loc.Sprintf("Ends:")+"</b> ") + "\n"
	t += "<b>" + loc.Sprintf("Location:") + "</b> <a href=\"" + eventUIMapURL(event) + "\">" + event.Location() + "</a>" + "\n"
	if event.MaxAttendees() > 0 {
		t += "<b>" + loc.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees()) + "\n"
	}
	if cost := eventUICostText(event, loc); cost != "" {
		t += cost + "\n"
	}
	if event.Notes() != "" {
		t += "<b>" + loc.Sprintf("Notes:") + "</b>\n" + event.Notes() + "\n"
	}

	people := gatherEventUIPeople(event)
	if people.err != nil {
		t += loc.Sprintf("Unable to get list of attending people.") + "\n"
	}

	// Fursuit walks display different event messages
	if event.Suitwalk() {
		t += classicPeopleSection(loc.Sprintf("Suiting: %v", people.cSuiting), people.suiting, event.HideNames())
		t += classicPeopleSection(loc.Sprintf("Photographers: %v", people.cPhoto), people.photo, event.HideNames())
		t += classicPeopleSection(loc.Sprintf("Spotting: %v", people.cGoing), people.going, event.HideNames())
	} else {
		t += classicPeopleSection(loc.Sprintf("Attending: %v", people.cGoing), people.going, event.HideNames())
	}
	if len(people.maybe) > 0 {
		t += classicNamesSection(loc.Sprintf("Maybe: %v", len(people.maybe)), people.maybe, event.HideNames())
	}
	if len(people.not) > 0 {
		t += classicNamesSection(loc.Sprintf("Can't make it: %v", len(people.not)), people.not, event.HideNames())
	}

	if status := eventUIStatusText(event, people, loc); status != "" {
		t += "\n<i>" + status + "</i>"
	}
	return strings.TrimRight(t, "\n")
}

// classicPeopleSection creates one of the lists of people coming to the event, with a heading, for a classic message.
func classicPeopleSection(heading string, list []eventUIPerson, hideNames bool) string {
	if len(list) == 0 {
		return ""
	}
	t := "\n<b>" + heading + "</b>\n"
	if hideNames {
		return t
	}
	return t + plainPeopleLines(list) + "\n"
}

// classicNamesSection creates a list of names with a heading for a classic message.
func classicNamesSection(heading string, names []string, hideNames bool) string {
	t := "\n<b>" + heading + "</b>\n"
	if hideNames {
		return t
	}
	return t + strings.Join(names, ", ") + "\n"
}

// plainPeopleLines writes out a list of people using only line breaks.
func plainPeopleLines(list []eventUIPerson) string {
	var lines []string
	for _, person := range list {
		lines = append(lines, " - "+person.label())
		for _, guest := range person.guests {
			lines = append(lines, fmt.Sprintf("   - <i>%v</i>", guest))
		}
	}
	return strings.Join(lines, "\n")
}

// makeEventUIRichHtml creates the event text for a rich message.
func (tgp *TGPlansBot) makeEventUIRichHtml(event dbInterface.DBEvent, loc *localizer.Localizer) string {
	return buildEventUIRichHtml(event, loc, time.Now())
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

// buildEventUIRichHtml creates the event text using rich message blocks. The current time decides how the date is written.
func buildEventUIRichHtml(event dbInterface.DBEvent, loc *localizer.Localizer, now time.Time) string {
	people := gatherEventUIPeople(event)

	t := "<h1>" + richInline(event.Name()) + "</h1>"
	t += "<h4>" + richInline(loc.Sprintf("Hosted by %v", event.OwnerName())) + "</h4>"

	// The event's own time is what matters for an event in person. The countdown is worked out by the viewer's app.
	dateText := loc.FormatShortDateRangeForLocale(event.DateTime(), event.EndDateTime(), now)
	countdown := fmt.Sprintf(`<tg-time unix="%d" format="r">%s</tg-time>`, event.DateTime().Unix(), loc.FormatShortDateForLocale(event.DateTime(), now))
	details := []string{
		"📅 " + dateText + " · " + countdown,
		`📍 <a href="` + eventUIMapURL(event) + `">` + richInline(event.Location()) + "</a>",
	}
	if event.MaxAttendees() > 0 && people.err == nil {
		if eventIsFull(event, people) {
			details = append(details, loc.Sprintf("👥 %d of %d spots taken · Full", people.coming(), event.MaxAttendees()))
		} else {
			details = append(details, loc.Sprintf("👥 %d of %d spots taken · %d left", people.coming(), event.MaxAttendees(), event.MaxAttendees()-people.coming()))
		}
	}
	t += "<p>" + strings.Join(details, "<br>") + "</p>"

	if cost := eventUICostText(event, loc); cost != "" {
		t += "<p>" + cost + "</p>"
	}
	if event.Notes() != "" {
		t += helpers.TelegramHtmlToRichQuote(event.Notes())
	}

	if people.err != nil {
		t += "<p>" + loc.Sprintf("Unable to get list of attending people.") + "</p>"
	} else {
		t += richPeopleSections(event, people, loc)
	}

	if status := eventUIStatusText(event, people, loc); status != "" {
		t += "<footer>" + status + "</footer>"
	}
	return t
}

// richInline makes text that was written for a classic message safe to use inside a single rich block.
func richInline(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "<br>")
}

// richPeopleSections creates all the lists of people for a rich message.
func richPeopleSections(event dbInterface.DBEvent, people eventUIResponses, loc *localizer.Localizer) string {
	hide := event.HideNames()
	plain := people.rows() > richListRowsLimit
	t := ""

	// Maybe and Can't make it are never left open when a list of people coming is collapsed.
	anyCollapsed := false
	section := func(heading string, list []eventUIPerson, count int) {
		collapsed := count > collapseListsOver
		anyCollapsed = anyCollapsed || collapsed
		t += richPeopleSection(heading, list, hide, collapsed, plain)
	}

	// An empty list invites people to respond, unless the event is closed and they can't.
	emptyHeading := func(heading string) string {
		if event.Closed() {
			return heading
		}
		return heading + " · " + loc.Sprintf("Be the first!")
	}

	// Fursuit walks display different event messages
	if event.Suitwalk() {
		if people.coming() == 0 {
			t += "<p><b>" + emptyHeading(loc.Sprintf("Suiting: %v", people.cSuiting)) + "</b></p>"
		}
		if len(people.suiting) > 0 {
			section(loc.Sprintf("Suiting: %v", people.cSuiting), people.suiting, people.cSuiting)
		}
		if len(people.photo) > 0 {
			section(loc.Sprintf("Photographers: %v", people.cPhoto), people.photo, people.cPhoto)
		}
		if len(people.going) > 0 {
			section(loc.Sprintf("Spotting: %v", people.cGoing), people.going, people.cGoing)
		}
	} else if len(people.going) == 0 {
		t += "<p><b>" + emptyHeading(loc.Sprintf("Attending: %v", people.cGoing)) + "</b></p>"
	} else {
		section(loc.Sprintf("Attending: %v", people.cGoing), people.going, people.cGoing)
	}

	if len(people.maybe) > 0 {
		t += richNamesSection(loc.Sprintf("Maybe: %v", len(people.maybe)), people.maybe, hide, anyCollapsed)
	}
	if len(people.not) > 0 {
		t += richNamesSection(loc.Sprintf("Can't make it: %v", len(people.not)), people.not, hide, anyCollapsed)
	}
	return t
}

// richPeopleSection creates one of the lists of people coming to the event, with a heading.
// A collapsed list is a details block. It has no 'open' attribute, so it starts closed.
func richPeopleSection(heading string, list []eventUIPerson, hideNames bool, collapsed bool, plain bool) string {
	heading = "<b>" + heading + "</b>"
	if hideNames {
		return "<p>" + heading + "</p>"
	}

	var body string
	if plain {
		body = helpers.TelegramHtmlToRichHtml(plainPeopleLines(list))
	} else {
		body = richPeopleList(list)
	}
	if collapsed {
		return "<details><summary>" + heading + "</summary>" + body + "</details>"
	}
	return "<p>" + heading + "</p>" + body
}

// richPeopleList creates a list of people, with any named guests in a list inside the person bringing them.
func richPeopleList(list []eventUIPerson) string {
	t := "<ul>"
	for _, person := range list {
		t += "<li>" + person.label()
		if len(person.guests) > 0 {
			t += "<ul>"
			for _, guest := range person.guests {
				t += "<li><i>" + guest + "</i></li>"
			}
			t += "</ul>"
		}
		t += "</li>"
	}
	return t + "</ul>"
}

// richNamesSection creates a list of names that shares a line with its heading, or a collapsed list if it is long.
func richNamesSection(heading string, names []string, hideNames bool, forceCollapse bool) string {
	heading = "<b>" + heading + "</b>"
	if hideNames {
		return "<p>" + heading + "</p>"
	}
	list := strings.Join(names, ", ")
	if forceCollapse || len(names) > collapseListsOver {
		return "<details><summary>" + heading + "</summary><p>" + list + "</p></details>"
	}
	return "<p>" + heading + " — " + list + "</p>"
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
				row = append(row, quickButton(emoji1+loc.Sprintf("+1"), fmt.Sprintf("attending:%v:1", event.ID())))
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

	if event.TotalCost() > 0 && !eventIsFunded(event) {

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
