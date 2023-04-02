package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"strconv"
	"strings"
)

const (
	EDIT_EVENTID     = "EDIT_EVENTID"
	EDIT_EVENT       = "EDIT_EVENT"
	EDIT_EVENTSTRING = "EDIT_EVENTSTRING"
	EDIT_EVENTNUMBER = "EDIT_EVENTNUMBER"
)

const GENERAL_ERROR = "A general error occurred."

// This handles one of the callback functions for when an 'edit' button is clicked.
func manage_clickEdit(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {

	// Format is: //edit:<id>:item_to_edit
	// TODO: Length check here
	data := strings.Split(cb.Data, ":")

	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}

	event, err := dbHelper.GetEvent(uint(eventId), int64(cb.From.ID))
	if err != nil {
		return
	}

	// Remember which event they are editing.
	usrInfo.SetData(EDIT_EVENTID, eventId)
	usrInfo.SetData(EDIT_EVENT, event)

	// What thing do they want to edit?

	// SIMPLE STRING EDIT
	switch data[2] {
	case "name":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Name, usrInfo.Locale.Sprintf("Specify the name of the event."))
	case "location":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Location, usrInfo.Locale.Sprintf(CHOOSE_LOCATION))
	case "hostedby":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.OwnerName, usrInfo.Locale.Sprintf("Specify the name of the person hosting the event."))
	case "notes":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Notes, usrInfo.Locale.Sprintf("Specify any additional notes you'd like to show about the event."))
	}

	// SIMPLE INTEGER
	switch data[2] {
	case "maxattend":
		editNumberItem(tg, usrInfo, int64(cb.From.ID), &event.MaxAttendees, usrInfo.Locale.Sprintf("Specify the maximum number of people that can attend.  Once the maximum is reached, users will no longer be able to click 'I'm Going'.\n\nTo disable, send a 0."))
	}

	// TOGGLES
	switch data[2] {
	case "sharing":
		toggleItem(tg, usrInfo, cb, &event.AllowShare, event)
	case "setmaybe":
		toggleItem(tg, usrInfo, cb, &event.DisableMaybe, event)
	}
}

func toggleItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, iValue *int, event *dbHelper.FurryPlans) {
	// Toggle this item
	*iValue = 1 - *iValue

	// Save the changes
	err := dbHelper.UpdateEvent(event.EventID, event)
	if err != nil {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf("error updating event: %v", err))
		_, _ = tg.Send(mObj)
		return
	}

	eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", cb.Message.MessageID)

}

// editStringItem puts them into a mode where they are editing a text item
func editStringItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *string, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSTRING, EditItem)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_STRING)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	_, _ = tg.Send(mObj)
}

// Called from when the mode above is finished
func edit_setString(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	editString, ok := usrInfo.GetData(EDIT_EVENTSTRING).(*string)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	htmlText := tg.ConvertEntitiesToHTML(text, msg.Entities)

	// Set the string to this value.  This should update it in the struct.
	*editString = htmlText

	//fmt.Println(htmlText)

	//fmt.Printf("Text: %#v\n", msg.Text)
	//fmt.Printf("Entities: %#v\n", msg.Entities)

	// Save the changes to the string.
	err := dbHelper.UpdateEvent(event.EventID, event)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0)
}

// editNumberItem puts them into a mode where they are editing a number item
func editNumberItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *int, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTNUMBER, EditItem)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_NUMBER)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	_, _ = tg.Send(mObj)
}

// Called from when the mode above is finished
func edit_setNumber(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	editNumber, ok := usrInfo.GetData(EDIT_EVENTNUMBER).(*int)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	// Convert the value to a number
	num, err := strconv.Atoi(text)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Please provide a valid number"))
		return
	}
	// Set the string to this value.  This should update it in the struct.
	*editNumber = num

	// Save the changes to the string.
	err = dbHelper.UpdateEvent(event.EventID, event)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0)
}
