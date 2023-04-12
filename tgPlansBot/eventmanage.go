package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
)

func initEventCommands(cmds *tgCommands.CommandList) {
	loc := localizer.FromLanguage("default") // not a real locale

	// General edit commands
	cmds.Add(tgCommands.Command{Command: "/myevents", Handler: listEvents, HelpText: loc.Sprintf("A list of your upcoming events")})
	cmds.Add(tgCommands.Command{Command: "/oldevents", Handler: listEventsOld, HelpText: loc.Sprintf("A list of all your events, old and new")})
	cmds.Add(tgCommands.Command{Command: "/edit", Handler: selectEvent, Underscore: true})

	// Create commands
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTNAME, Handler: create_SetName})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTDATE, Handler: create_SetDate})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTTIME, Handler: create_SetTime})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_CREATE_EVENTLOCATION, Handler: create_SetLocation})

	// Edit commands
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_STRING, Handler: edit_setString})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_NUMBER, Handler: edit_setNumber})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_CHOICE, Handler: edit_setChoice})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_DATE, Handler: edit_setDate})
	cmds.Add(tgCommands.Command{Mode: userManager.MODE_EDIT_TIME, Handler: edit_setTime})

	cmds.AddCB(tgCommands.Callback{DataPrefix: "calen", Mode: userManager.MODE_CREATE_EVENTDATE, Handler: create_ClickDate})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "time", Mode: userManager.MODE_CREATE_EVENTTIME, Handler: create_ClickTime})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "calen", Mode: userManager.MODE_EDIT_DATE, Handler: edit_ClickDate})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "time", Mode: userManager.MODE_EDIT_TIME, Handler: edit_ClickTime})
	cmds.AddCB(tgCommands.Callback{DataPrefix: "edit", Handler: manage_clickEdit})
}

// eventDetails displays the details about the event to the user.
// This lets them edit properties of the event, or share it out.
func eventDetails(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, event dbInterface.DBEvent, topMsg string, editInPlace int, showAdvancedButtons bool) {

	loc := localizer.FromLanguage(event.Language())

	// Start with an optional message at the top.
	t := topMsg

	t += "<b>" + loc.Sprintf("Name:") + "</b> " + event.Name() + "\n"
	t += "<b>" + loc.Sprintf("Date:") + "</b> " + loc.FormatDateForLocale(event.DateTime()) + "\n"
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

	var mObj tgbotapi.Chattable

	var buttons tgbotapi.InlineKeyboardMarkup
	if showAdvancedButtons {
		buttons = eventAdvancedButtons(event, loc)
	} else {
		buttons = eventEditButtons(event, loc)
	}
	if editInPlace != 0 {
		mObj2 := tgbotapi.NewEditMessageText(chatId, editInPlace, t)
		mObj2.ParseMode = tgWrapper.ParseModeHtml
		mObj2.ReplyMarkup = &buttons
		mObj2.DisableWebPagePreview = true
		mObj = mObj2
	} else {
		mObj2 := tgbotapi.NewMessage(chatId, t)
		mObj2.ParseMode = tgWrapper.ParseModeHtml
		mObj2.ReplyMarkup = buttons
		mObj2.DisableWebPagePreview = true
		mObj = mObj2
	}

	_, err := tg.Request(mObj)
	if err != nil {
		log.Println(err)
	}
}

// listEvents will list all the events the user has created that are not too far in the past.
func listEvents(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	listEventsReal(tg, usrInfo, msg, false)
}

// listEvents will list all the events the user has created that are not too far in the past.
func listEventsOld(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	listEventsReal(tg, usrInfo, msg, true)
}

func listEventsReal(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, includeOld bool) {
	events, err := db.GetEvents(msg.Chat.ID, includeOld)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Error listing events: %v", err))
		return
	}

	t := ""
	for _, event := range events {
		t += fmt.Sprintf("/edit_%v - %v\n", event.ID(), event.Name())
	}
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf("Select an event to edit:\n%v", t))
	mObj.ParseMode = tgWrapper.ParseModeHtml
	_, err = tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

func selectEvent(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Find this event.
	eventId, err := strconv.Atoi(text)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Unable to parse event ID: %v", err))
		return
	}

	// Load the details about the event from the database.
	event, err := db.GetEvent(uint(eventId), msg.Chat.ID)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Event not found"))
		return
	}

	// Display the event information now.
	eventDetails(tg, usrInfo, msg.Chat.ID, event, "", 0, false)
}
