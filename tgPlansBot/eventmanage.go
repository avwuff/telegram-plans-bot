package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
)

func initEventCommands(cmds *tgCommands.CommandList) {
	// General edit commands
	cmds.Add(tgCommands.Command{Command: "/myevents", Handler: listEvents})
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
func eventDetails(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, eventId uint, topMsg string, editInPlace int, showAdvancedButtons bool) {

	// Load the details about the event from the database.
	event, locOverride, err := dbHelper.GetEvent(eventId, chatId)
	if err != nil {
		mObj := tgbotapi.NewMessage(chatId, usrInfo.Locale.Sprintf("Event not found"))
		_, _ = tg.Send(mObj)
		return
	}

	// Override the locale if the event specifies a different one
	if locOverride != nil {
		usrInfo.Locale = locOverride
	}

	// Start with an optional message at the top.
	t := topMsg

	t += "<b>" + usrInfo.Locale.Sprintf("Name:") + "</b> " + event.Name + "\n"
	t += "<b>" + usrInfo.Locale.Sprintf("Date:") + "</b> " + usrInfo.Locale.FormatDate(event.DateTime.Time) + "\n"
	t += "<b>" + usrInfo.Locale.Sprintf("Location:") + "</b> " + event.Location + "\n"
	t += "<b>" + usrInfo.Locale.Sprintf("Hosted By:") + "</b> " + event.OwnerName + "\n"
	if event.Suitwalk == 1 {
		t += "<b>" + usrInfo.Locale.Sprintf("Suitwalk:") + "</b> " + usrInfo.Locale.Sprintf("Yes") + "\n"
	}
	if event.MaxAttendees > 0 {
		t += "<b>" + usrInfo.Locale.Sprintf("Max Attendees:") + "</b> " + fmt.Sprintf("%v", event.MaxAttendees) + "\n"
	}
	if event.Language != "" {
		t += "<b>" + usrInfo.Locale.Sprintf("Language:") + "</b> " + fmt.Sprintf("%v", event.Language) + "\n"
	}
	if event.Notes != "" {
		t += "<b>" + usrInfo.Locale.Sprintf("Notes:") + "</b>\n" + event.Notes + "\n"
	}

	var mObj tgbotapi.Chattable

	var buttons tgbotapi.InlineKeyboardMarkup
	if showAdvancedButtons {
		buttons = eventAdvancedButtons(event, usrInfo.Locale)
	} else {
		buttons = eventEditButtons(event, usrInfo.Locale)
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

	_, err = tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// listEvents will list all the events the user has created that are not too far in the past.
func listEvents(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	events, err := dbHelper.GetEvents(msg.Chat.ID, false)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Error listing events: %v", err))
		return
	}

	t := ""
	for _, event := range events {
		t += fmt.Sprintf("/edit_%v - %v\n", event.EventID, event.Name)
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
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Unable to parse event ID", err))
		return
	}

	// Display the event information now.
	eventDetails(tg, usrInfo, msg.Chat.ID, uint(eventId), "", 0, false)
}
