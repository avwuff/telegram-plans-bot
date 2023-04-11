package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	EDIT_EVENTID      = "EDIT_EVENTID"
	EDIT_EVENT        = "EDIT_EVENT"
	EDIT_EVENTCOLNAME = "EDIT_EVENTCOLNAME"
	EDIT_EVENTDATE    = "EDIT_EVENTDATE"
	EDIT_EVENTSTRING  = "EDIT_EVENTSTRING"
	EDIT_EVENTNUMBER  = "EDIT_EVENTNUMBER"
	EDIT_EVENTCHOICES = "EDIT_EVENTCHOICES"
)

const GENERAL_ERROR = "A general error occurred."

// This handles one of the callback functions for when an 'edit' button is clicked.
func manage_clickEdit(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {

	// Format is: //edit:<id>:item_to_edit
	data := strings.Split(cb.Data, ":")
	if len(data) != 3 {
		return
	}

	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}

	event, locOverride, err := dbHelper.GetEvent(uint(eventId), int64(cb.From.ID))
	if err != nil {
		return
	}

	// Override the locale if the event specifies a different one
	if locOverride != nil {
		usrInfo.Locale = locOverride
	}

	// Remember which event they are editing.
	usrInfo.SetData(EDIT_EVENTID, eventId)
	usrInfo.SetData(EDIT_EVENT, event)

	// What thing do they want to edit?

	// SIMPLE STRING EDIT
	switch data[2] {
	case "name":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Name, "EventName", usrInfo.Locale.Sprintf("Specify the name of the event."), false)
	case "location":
		// BUG: This should really be the same text as the const CHOOSE_LOCATION, but it makes GOTEXT crash when you use the const here.
		// Go figure.
		msg := usrInfo.Locale.Sprintf("Where does the event take place?  Specify the name or address as you might type into Google Maps.")
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Location, "EventLocation", msg, false)
	case "hostedby":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.OwnerName, "ownerName", usrInfo.Locale.Sprintf("Specify the name of the person hosting the event."), false)
	case "notes":
		editStringItem(tg, usrInfo, int64(cb.From.ID), &event.Notes, "Notes", usrInfo.Locale.Sprintf("Specify any additional notes you'd like to show about the event."), true)

	// SPECIAL EDITORS
	case "date":
		editDateItem(tg, usrInfo, int64(cb.From.ID), &event.DateTime.Time, "EventDateTime", usrInfo.Locale.Sprintf("Specify the date on which this event takes place:"))
	case "time":
		editTimeItem(tg, usrInfo, int64(cb.From.ID), &event.DateTime.Time, "EventDateTime", usrInfo.Locale.Sprintf("Specify the time at which this event takes place:"))

	// SIMPLE INTEGER
	case "maxattend":
		editNumberItem(tg, usrInfo, int64(cb.From.ID), &event.MaxAttendees, "MaxAttendees", usrInfo.Locale.Sprintf("Specify the maximum number of people that can attend.  Once the maximum is reached, users will no longer be able to click 'I'm Going'.\n\nTo disable, send a 0."))

	// CHOICE
	case "language":
		editChoiceItem(tg, usrInfo, int64(cb.From.ID), &event.Language, "Language", usrInfo.Locale.Sprintf("Choose the display language for this event."), localizer.GetLanguageChoicesMap())

	// TOGGLES
	case "sharing":
		toggleItem(tg, usrInfo, cb, &event.AllowShare, "AllowShare", event)
	case "setmaybe":
		toggleItem(tg, usrInfo, cb, &event.DisableMaybe, "DisableMaybe", event)
	case "suitwalk":
		toggleItem(tg, usrInfo, cb, &event.Suitwalk, "Suitwalk", event)

	// COMMANDS
	case "advanced":
		// Show the advanced options
		showAdvancedPanel(tg, usrInfo, cb, event)
	case "back":
		// Just go back to the main panel
		eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", cb.Message.MessageID, false)
	}
}

// TIME SELECTION
// editTimeItem displays the time selector.
func editTimeItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *time.Time, columnName string, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTDATE, EditItem)
	usrInfo.SetData(EDIT_EVENTCOLNAME, columnName)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_TIME)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = createTimeSelection(*EditItem, usrInfo.Locale)
	_, _ = tg.Send(mObj)
}

func edit_ClickTime(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which time element did they click?
	// We update the message as needed.
	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}
	editTime, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}

	edit := tgbotapi.NewEditMessageText(int64(cb.From.ID), cb.Message.MessageID, cb.Message.Text)

	var finished bool
	*editTime, finished = processTimeClicks(*editTime, cb.Data)

	// Send the calendar again.
	if !finished {
		timeButtons := createTimeSelection(*editTime, usrInfo.Locale)
		edit.ReplyMarkup = &timeButtons
	} else {
		edit.Text = usrInfo.Locale.Sprintf("Time selected: %v", usrInfo.Locale.FormatTimeForLocale(*editTime))

		// Save the value
		err := event.UpdateEvent(colName)
		if err != nil {
			mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf("error updating event: %v", err))
			_, _ = tg.Send(mObj)
			return
		}

		// switch back to normal mode and display the event details
		usrInfo.SetMode(userManager.MODE_DEFAULT)
		eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", 0, false)
		updateEventUIAllPostings(tg, event, "")
	}

	_, err := tg.Send(edit)
	if err != nil {
		log.Println(err)
	}
}

// Called from when the mode above is finished
func edit_setTime(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	editTime, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	selTime, err := time.ParseInLocation("15:04", text, editTime.Location())
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Could not parse the time you provided. Please send it in the format 22:03."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	*editTime = changeJustTime(*editTime, selTime)

	// Save the changes to the string.
	err = event.UpdateEvent(colName)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0, false)
	updateEventUIAllPostings(tg, event, "")
}

// editDateItem displays the date select calendar.
func editDateItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *time.Time, columnName string, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTDATE, EditItem)
	usrInfo.SetData(EDIT_EVENTCOLNAME, columnName)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_DATE)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = createCalendar(*EditItem, usrInfo.Locale, *EditItem)
	_, _ = tg.Send(mObj)
}

// DATE SELECTION
func edit_ClickDate(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which date element did they click?
	// We update the message as needed.

	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}
	editDate, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
	if !ok {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tg.Send(mObj)
		return
	}

	edit := tgbotapi.NewEditMessageText(int64(cb.From.ID), cb.Message.MessageID, cb.Message.Text)

	newDate, finished := processDateClicks(*editDate, cb.Data)
	*editDate = changeJustDate(*editDate, newDate)

	// Send the calendar again.
	if !finished {
		calen := createCalendar(*editDate, usrInfo.Locale, *editDate)
		edit.ReplyMarkup = &calen
	} else {
		edit.Text = usrInfo.Locale.Sprintf("Date selected: %v", usrInfo.Locale.FormatTimeForLocale(*editDate))

		// Save the value
		err := event.UpdateEvent(colName)
		if err != nil {
			mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf("error updating event: %v", err))
			_, _ = tg.Send(mObj)
			return
		}

		// switch back to normal mode and display the event details
		usrInfo.SetMode(userManager.MODE_DEFAULT)
		eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", 0, false)
		updateEventUIAllPostings(tg, event, "")
	}
	_, err := tg.Send(edit)
	if err != nil {
		log.Println(err)
	}
}

// Called from when the mode above is finished
func edit_setDate(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(*dbHelper.FurryPlans)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	editDate, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	selDate, err := time.ParseInLocation(layoutISO, text, editDate.Location())
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Could not parse the date you provided. Please send it in the format YYYY-MM-DD."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	*editDate = changeJustDate(*editDate, selDate)

	// Save the changes to the string.
	err = event.UpdateEvent(colName)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0, false)
	updateEventUIAllPostings(tg, event, "")
}

func changeJustDate(fullDate time.Time, newDate time.Time) time.Time {
	return time.Date(newDate.Year(), newDate.Month(), newDate.Day(), fullDate.Hour(), fullDate.Minute(), fullDate.Second(), 0, fullDate.Location())
}
func changeJustTime(fullDate time.Time, newDate time.Time) time.Time {
	return time.Date(fullDate.Year(), fullDate.Month(), fullDate.Day(), newDate.Hour(), newDate.Minute(), newDate.Second(), 0, fullDate.Location())
}

func showAdvancedPanel(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, event *dbHelper.FurryPlans) {

	eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", cb.Message.MessageID, true)

}

func toggleItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, iValue *int, columnName string, event *dbHelper.FurryPlans) {
	// Toggle this item
	*iValue = 1 - *iValue

	// Save the changes
	err := event.UpdateEvent(columnName)
	if err != nil {
		mObj := tgbotapi.NewMessage(int64(cb.From.ID), usrInfo.Locale.Sprintf("error updating event: %v", err))
		_, _ = tg.Send(mObj)
		return
	}

	eventDetails(tg, usrInfo, int64(cb.From.ID), event.EventID, "", cb.Message.MessageID, false)
	updateEventUIAllPostings(tg, event, "")
}

func editChoiceItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *string, columnName string, prompt string, choices map[string]string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSTRING, EditItem)
	usrInfo.SetData(EDIT_EVENTCHOICES, choices)
	usrInfo.SetData(EDIT_EVENTCOLNAME, columnName)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_CHOICE)

	// Build a keyboard of the choices
	var keyboard [][]tgbotapi.KeyboardButton
	for _, choice := range choices {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(choice),
		))
	}

	choiceKeyboard := tgbotapi.ReplyKeyboardMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		Keyboard:        keyboard,
	}

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = choiceKeyboard
	_, _ = tg.Send(mObj)
}

// Called from when the mode above is finished
func edit_setChoice(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

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

	// Get the original list of choices
	choices, ok := usrInfo.GetData(EDIT_EVENTCHOICES).(map[string]string)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
	if !ok {
		quickReply(tg, msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	// The text has to be one of the choices.
	found := ""
	for key, choice := range choices {
		if choice == text {
			found = key
			break
		}
	}

	if found == "" {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("The value you provided is not one of the choices."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	*editString = found

	// Save the changes to the string.
	err := event.UpdateEvent(colName)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0, false)
	updateEventUIAllPostings(tg, event, "")
}

// editStringItem puts them into a mode where they are editing a text item
func editStringItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *string, columnName string, prompt string, sendExisting bool) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSTRING, EditItem)
	usrInfo.SetData(EDIT_EVENTCOLNAME, columnName)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_STRING)

	// Optionally this function can also send the existing value so the use can copy it easily.
	if sendExisting && *EditItem != "" {
		mObj := tgbotapi.NewMessage(chatId, *EditItem)
		mObj.ParseMode = tgWrapper.ParseModeHtml
		mObj.DisableWebPagePreview = true
		_, _ = tg.Send(mObj)
	}

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
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
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
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
	err := event.UpdateEvent(colName)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0, false)
	updateEventUIAllPostings(tg, event, "")
}

// editNumberItem puts them into a mode where they are editing a number item
func editNumberItem(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, EditItem *int, columnName string, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTNUMBER, EditItem)
	usrInfo.SetData(EDIT_EVENTCOLNAME, columnName)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_NUMBER)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
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
	colName, ok := usrInfo.GetData(EDIT_EVENTCOLNAME).(string)
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
	err = event.UpdateEvent(colName)
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, event.EventID, "", 0, false)
	updateEventUIAllPostings(tg, event, "")
}
