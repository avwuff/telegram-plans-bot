package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
)

func (tgp *TGPlansBot) initEventCommands() {
	loc := localizer.FromLanguage("default") // not a real locale

	// General edit commands
	tgp.cmds.Add(tgCommands.Command{Command: "/myevents", Handler: tgp.listEvents, HelpText: loc.Sprintf("A list of your upcoming events")})
	tgp.cmds.Add(tgCommands.Command{Command: "/oldevents", Handler: tgp.listEventsOld, HelpText: loc.Sprintf("A list of all your events, old and new")})
	tgp.cmds.Add(tgCommands.Command{Command: "/edit", Handler: tgp.selectEvent, Underscore: true})
	tgp.cmds.Add(tgCommands.Command{Command: "/cost", Handler: tgp.costDashboard, Underscore: true})

	// Create commands
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTNAME, Handler: tgp.create_SetName})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTDATE, Handler: tgp.create_SetDate})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTTIME, Handler: tgp.create_SetTime})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTLOCATION, Handler: tgp.create_SetLocation})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_SETNOTES, Handler: tgp.create_SetNotes})

	// Edit commands
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_STRING, Handler: tgp.edit_setString})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_NUMBER, Handler: tgp.edit_setNumber})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_CHOICE, Handler: tgp.edit_setChoice})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_DATE, Handler: tgp.edit_setDate})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_TIME, Handler: tgp.edit_setTime})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_PICTURE, Handler: tgp.edit_setPicture, SpecialMode: "photo"})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_COSTS1, Handler: tgp.edit_setTotalCost})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_COSTS2, Handler: tgp.edit_setCostInfo})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_PUBLIC, Handler: tgp.edit_setPublic, SpecialMode: "public"})

	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "calen", Mode: userManager.MODE_CREATE_EVENTDATE, Handler: tgp.create_ClickDate})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "time", Mode: userManager.MODE_CREATE_EVENTTIME, Handler: tgp.create_ClickTime})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "calen", Mode: userManager.MODE_EDIT_DATE, Handler: tgp.edit_ClickDate})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "time", Mode: userManager.MODE_EDIT_TIME, Handler: tgp.edit_ClickTime})
	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "edit", Handler: tgp.manage_clickEdit})
}

// eventDetails displays the details about the event to the user.
// This lets them edit properties of the event, or share it out.
func (tgp *TGPlansBot) eventDetails(usrInfo *userManager.UserInfo, chatId int64, event dbInterface.DBEvent, topMsg string, editInPlace int, showAdvancedButtons bool) {

	loc := localizer.FromLanguage(event.Language())

	// Start with an optional message at the top.
	t := topMsg

	t += "<b>" + loc.Sprintf("Name:") + "</b> " + event.Name() + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDateForLocale(event.DateTime()) + "\n"

	// Does it end at a different time?
	if !event.EndDateTime().IsZero() && event.EndDateTime() != event.DateTime() {
		// If just the time is different, then we just present the time.  Otherwise, we present the day and the time.
		t += "<b>" + loc.Sprintf("Ends at:") + "</b> " + loc.FormatEndDateForLocale(event.DateTime(), event.EndDateTime()) + "\n"
	}

	t += "<b>" + loc.Sprintf("Location:") + "</b> " + event.Location() + "\n"
	t += "<b>" + loc.Sprintf("Hosted By:") + "</b> " + event.OwnerName() + "\n"
	if event.Suitwalk() {
		t += "<b>" + loc.Sprintf("Suitwalk:") + "</b> " + loc.Sprintf("Yes") + "\n"
	}
	if event.MaxAttendees() > 0 {
		t += "<b>" + loc.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees()) + "\n"
	}
	if event.Language() != "" {
		t += "<b>" + loc.Sprintf("Language:") + "</b> " + fmt.Sprintf("%v", event.Language()) + "\n"
	}
	if event.Notes() != "" {
		t += "<b>" + loc.Sprintf("Notes:") + "</b>\n" + event.Notes() + "\n"
	}
	isPublic, _, _ := event.Public()
	if isPublic {
		t += "\n" + loc.Sprintf("🌎 Event is listed in public directory") + "\n"
	}

	if event.PictureURL() != "" {
		t += "\n<i>" + loc.Sprintf("🖼 Event includes a picture") + "</i>\n"
	}
	if event.TotalCost() > 0 {
		t += "\n<i>" + strings.ReplaceAll(loc.Sprintf("💰 Accepting donations to cover cost of %v. Click XXXX for totals", event.TotalCost()), "XXXX", fmt.Sprintf("/cost_%d", event.ID())) + "</i>\n"
	}

	var buttons tgbotapi.InlineKeyboardMarkup
	if showAdvancedButtons {
		buttons = eventAdvancedButtons(event, loc)
	} else {
		buttons = eventEditButtons(event, loc)
	}
	if editInPlace != 0 {
		mObj := tgbotapi.NewEditMessageText(chatId, editInPlace, t)
		mObj.ParseMode = ParseModeHtml
		mObj.ReplyMarkup = &buttons
		mObj.LinkPreviewOptions.IsDisabled = true
		_, err := tgp.tg.Request(mObj)
		if err != nil {
			log.Println(err)
		}
	} else {
		mObj := tgbotapi.NewMessage(chatId, t)
		mObj.ParseMode = ParseModeHtml
		mObj.ReplyMarkup = buttons
		mObj.LinkPreviewOptions.IsDisabled = true
		_, err := tgp.tg.Send(mObj)
		if err != nil {
			log.Println(err)
		}
	}

}

// listEvents will list all the events the user has created that are not too far in the past.
func (tgp *TGPlansBot) listEvents(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	tgp.listEventsReal(usrInfo, msg, false)
}

// listEvents will list all the events the user has created that are not too far in the past.
func (tgp *TGPlansBot) listEventsOld(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	tgp.listEventsReal(usrInfo, msg, true)
}

func (tgp *TGPlansBot) listEventsReal(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, includeOld bool) {
	events, err := tgp.db.GetEvents(msg.Chat.ID, includeOld)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Error listing events: %v", err))
		return
	}

	t := ""
	for _, event := range events {
		t += fmt.Sprintf("/edit_%v - %v\n", event.ID(), event.Name())
	}
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf("Select an event to edit:\n%v", t))
	mObj.ParseMode = ParseModeHtml
	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) selectEvent(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Find this event.
	eventId, err := strconv.Atoi(text)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Unable to parse event ID: %v", err))
		return
	}

	// Load the details about the event from the database.
	event, err := tgp.db.GetEvent(uint(eventId), msg.Chat.ID)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Event not found"))
		return
	}

	// Display the event information now.
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
}

func (tgp *TGPlansBot) costDashboard(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Find this event.
	eventId, err := strconv.Atoi(text)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Unable to parse event ID: %v", err))
		return
	}

	// Load the details about the event from the database.
	event, err := tgp.db.GetEvent(uint(eventId), msg.Chat.ID)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Event not found"))
		return
	}

	donors, err := event.GetDonors()
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Error getting donors"))
		return
	}

	// Display information about the cost of the event here.
	loc := localizer.FromLanguage(event.Language())

	// Start with an optional message at the top.
	t := "<b>" + loc.Sprintf("This event is collecting donations in the amount of %d."+"</b>", event.TotalCost()) + "\n\n"

	t += "<b>" + loc.Sprintf("Collected so far:") + "</b>\n"
	if len(donors) == 0 {
		t += "<i>" + loc.Sprintf("No donations collected yet") + "</i>\n"
	} else {
		for _, donor := range donors {
			t += fmt.Sprintf(` - <a href="tg://user?id=%v">%v</a>: %.2f`+"\n", donor.UserID, donor.UserName, donor.Amount)
		}
	}
	t += "\n"
	t += "<b>" + loc.Sprintf("Information shown to donors:") + "</b>\n"
	t += event.CostInfo()

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("💰 Edit recovery information"), fmt.Sprintf("edit:%v:costs", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("Back to Event"), fmt.Sprintf("edit:%v:back", event.ID())))
	buttons = append(buttons, row)

	buttonsMarkup := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	mObj := tgbotapi.NewMessage(msg.Chat.ID, t)
	mObj.ParseMode = ParseModeHtml
	mObj.ReplyMarkup = buttonsMarkup
	mObj.LinkPreviewOptions.IsDisabled = true
	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}
