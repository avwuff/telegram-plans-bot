package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EDIT_EVENTID               = "EDIT_EVENTID"
	EDIT_EVENT                 = "EDIT_EVENT"
	EDIT_EVENTDATE             = "EDIT_EVENTDATE"
	EDIT_EVENTSTRING           = "EDIT_EVENTSTRING"
	EDIT_EVENTNUMBER           = "EDIT_EVENTNUMBER"
	EDIT_EVENTCHOICES          = "EDIT_EVENTCHOICES"
	EDIT_EVENTSETFUNC          = "EDIT_EVENTSETFUNC"
	EDIT_EVENT_RETURN_ADVANCED = "EDIT_EVENT_RETURN_ADVANCED" // whether or not to return to the 'advanced' view after making the choice
)

const GENERAL_ERROR = "A general error occurred."

type setStringFunc func(t string) error
type setIntFunc func(t int) error
type setBoolFunc func(t bool) error
type setTimeFunc func(t time.Time) error
type setPublicFunc func(v bool, lat float64, lon float64) error

// This handles one of the callback functions for when an 'edit' button is clicked.
func (tgp *TGPlansBot) manage_clickEdit(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {

	// Format is: //edit:<id>:item_to_edit
	data := strings.Split(cb.Data, ":")
	if len(data) != 3 {
		return
	}

	eventId, err := strconv.Atoi(data[1])
	if err != nil {
		return
	}

	event, err := tgp.db.GetEvent(uint(eventId), cb.From.ID)
	if err != nil {
		return
	}

	loc := localizer.FromLanguage(event.Language())

	// Remember which event they are editing.
	usrInfo.SetData(EDIT_EVENTID, eventId)
	usrInfo.SetData(EDIT_EVENT, event)

	// What thing do they want to edit?
	go tgp.answerCallback(cb, "")

	// SIMPLE STRING EDIT
	switch data[2] {
	case "name":
		tgp.editStringItem(usrInfo, cb.From.ID, event.Name(), event.SetName, loc.Sprintf("Specify the name of the event."), false)
	case "location":
		// BUG: This should really be the same text as the const CHOOSE_LOCATION, but it makes GOTEXT crash when you use the const here.
		// Go figure.
		msg := loc.Sprintf("Where does the event take place?  Specify the name or address as you might type into Google Maps.")
		tgp.editStringItem(usrInfo, cb.From.ID, event.Location(), event.SetLocation, msg, false)
	case "hostedby":
		tgp.editStringItem(usrInfo, cb.From.ID, event.OwnerName(), event.SetOwnerName, loc.Sprintf("Specify the name of the person hosting the event."), false)
	case "notes":
		tgp.editStringItem(usrInfo, cb.From.ID, event.Notes(), event.SetNotes, loc.Sprintf("Specify any additional notes you'd like to show about the event."), true)

	// PICTURES
	case "picture":
		tgp.editPictureItem(usrInfo, cb.From.ID, event.PictureURL(), event.SetPictureURL, loc.Sprintf("Send me a picture that will be included with your event."), loc.Sprintf("Current picture:"))

	// PUBLIC
	case "public":
		tgp.editPublicItem(usrInfo, cb.From.ID, event.SetPublic, loc.Sprintf("Including your event in our public event directory makes it available to anyone in the local area.  Anyone can choose to attend or share your event.  We recommend you use this feature only for large events, as attendance can be hard to control.\n\nIf you would like to share your event in our directories, send a Location pin (usually via the 📎 menu) that is close to where the event will take place. This allows nearby people to find your event.\n\nLocals will be able to use the command /nearby to see events near them."))
	case "notpublic": // Make the event no longer public
		tgp.toggleItem(usrInfo, cb, true /* true so it toggles to false */, event.SetPublicOnly, event, false)

	// SPECIAL EDITORS
	case "date":
		tgp.editDateItem(usrInfo, cb.From.ID, event.DateTime(), event.SetDateTime, loc.Sprintf("Specify the date on which this event takes place:"), loc)
	case "time":
		tgp.editTimeItem(usrInfo, cb.From.ID, event.DateTime(), event.SetDateTime, loc.Sprintf("Specify the time at which this event takes place:"), loc)

	case "enddate":
		if event.EndDateTime().IsZero() { // If it is unset, make it the same as the event date & time.
			event.SetEndDateTime(event.DateTime())
		}
		tgp.editDateItem(usrInfo, cb.From.ID, event.EndDateTime(), event.SetEndDateTime, loc.Sprintf("Specify the date when this event ends:"), loc)
	case "endtime":
		if event.EndDateTime().IsZero() {
			event.SetEndDateTime(event.DateTime())
		}
		tgp.editTimeItem(usrInfo, cb.From.ID, event.EndDateTime(), event.SetEndDateTime, loc.Sprintf("Specify the time when this event ends:"), loc)

	// SIMPLE INTEGER
	case "maxattend":
		tgp.editNumberItem(usrInfo, cb.From.ID, event.MaxAttendees(), event.SetMaxAttendees, loc.Sprintf("Specify the maximum number of people that can attend.  Once the maximum is reached, users will no longer be able to click 'I'm Going'.\n\nTo disable, send a 0."))

	// CHOICE
	case "language":
		tgp.editChoiceItem(usrInfo, cb.From.ID, event.SetLanguage, loc.Sprintf("Choose the display language for this event."), localizer.GetLanguageChoicesList(), true)
	case "timezone":
		tgp.editChoiceItem(usrInfo, cb.From.ID, event.SetTimeZone, loc.Sprintf("Choose the time zone for this event."), localizer.GetTimeZoneChoicesList(), true)

	// TOGGLES
	case "sharing":
		tgp.toggleItem(usrInfo, cb, event.SharingAllowed(), event.SetSharingAllowed, event, false)
	case "setmaybe":
		tgp.toggleItem(usrInfo, cb, event.DisableMaybe(), event.SetDisableMaybe, event, false)
	case "suitwalk":
		tgp.toggleItem(usrInfo, cb, event.Suitwalk(), event.SetSuitwalk, event, true)
	case "hidenames":
		tgp.toggleItem(usrInfo, cb, event.HideNames(), event.SetHideNames, event, true)
	case "close": // When the event is no longer accepting attendees
		tgp.toggleItem(usrInfo, cb, false, event.SetClosed, event, false)
	case "reopen": // Reopen after closing
		tgp.toggleItem(usrInfo, cb, true, event.SetClosed, event, false)
	case "maxguests":
		// Cycle between 0, 1, 2, 3
		set := event.MaxGuests() + 1
		if set > 3 {
			set = 0
		}
		tgp.directSetItem(usrInfo, cb, set, event.SetMaxGuests, event, true)

	// COMMANDS
	case "advanced":
		// Show the advanced options
		tgp.showAdvancedPanel(usrInfo, cb, event)
	case "back":
		// Just go back to the main panel
		tgp.eventDetails(usrInfo, cb.From.ID, event, "", cb.Message.MessageID, false)
	}

}

// TIME SELECTION
// editTimeItem displays the time selector.
func (tgp *TGPlansBot) editTimeItem(usrInfo *userManager.UserInfo, chatId int64, EditItem time.Time, SetFunc setTimeFunc, prompt string, loc *localizer.Localizer) {
	// Store a pointer to the string we are trying to set.
	tempDate := &EditItem
	usrInfo.SetData(EDIT_EVENTDATE, tempDate)
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_TIME)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = createTimeSelection(EditItem, loc)
	_, _ = tgp.tg.Send(mObj)
}

func (tgp *TGPlansBot) edit_ClickTime(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which time element did they click?
	// We update the message as needed.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}
	editTime, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setTimeFunc)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}

	edit := tgbotapi.NewEditMessageText(cb.From.ID, cb.Message.MessageID, cb.Message.Text)

	var finished bool
	*editTime, finished = processTimeClicks(*editTime, cb.Data)

	loc := localizer.FromLanguage(event.Language())

	// Send the calendar again.
	if !finished {
		timeButtons := createTimeSelection(*editTime, loc)
		edit.ReplyMarkup = &timeButtons
	} else {
		edit.Text = loc.Sprintf("Time selected: %v", loc.FormatTimeForLocale(*editTime))

		// Save the value
		err := setFunc(*editTime)
		if err != nil {
			mObj := tgbotapi.NewMessage(cb.From.ID, loc.Sprintf("error updating event: %v", err))
			_, _ = tgp.tg.Send(mObj)
			return
		}

		// switch back to normal mode and display the event details
		usrInfo.SetMode(userManager.MODE_DEFAULT)
		tgp.eventDetails(usrInfo, cb.From.ID, event, "", 0, false)
		tgp.updateEventUIAllPostings(event)
	}

	_, err := tgp.tg.Request(edit)
	if err != nil {
		log.Println(err)
	}
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setTime(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	editTime, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setTimeFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	selTime, err := time.ParseInLocation("15:04", text, editTime.Location())
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Could not parse the time you provided. Please send it in the format 22:03."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	*editTime = changeJustTime(*editTime, selTime)

	// Save the changes to the string.
	err = setFunc(*editTime)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}

// editDateItem displays the date select calendar.
func (tgp *TGPlansBot) editDateItem(usrInfo *userManager.UserInfo, chatId int64, EditItem time.Time, SetFunc setTimeFunc, prompt string, loc *localizer.Localizer) {
	// Store a pointer to the string we are trying to set.
	tempDate := &EditItem
	usrInfo.SetData(EDIT_EVENTDATE, tempDate)
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_DATE)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = createCalendar(EditItem, loc, EditItem)
	_, _ = tgp.tg.Send(mObj)
}

// DATE SELECTION
func (tgp *TGPlansBot) edit_ClickDate(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which date element did they click?
	// We update the message as needed.

	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}
	editDate, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setTimeFunc)
	if !ok {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		_, _ = tgp.tg.Send(mObj)
		return
	}

	edit := tgbotapi.NewEditMessageText(cb.From.ID, cb.Message.MessageID, cb.Message.Text)

	loc := localizer.FromLanguage(event.Language())

	newDate, finished := processDateClicks(*editDate, cb.Data)
	*editDate = changeJustDate(*editDate, newDate)

	// Send the calendar again.
	if !finished {
		calen := createCalendar(*editDate, loc, *editDate)
		edit.ReplyMarkup = &calen
	} else {
		edit.Text = loc.Sprintf("Date selected: %v", loc.FormatDateForLocale(*editDate))

		// Save the value
		err := setFunc(*editDate)
		if err != nil {
			mObj := tgbotapi.NewMessage(cb.From.ID, loc.Sprintf("error updating event: %v", err))
			_, _ = tgp.tg.Send(mObj)
			return
		}

		// switch back to normal mode and display the event details
		usrInfo.SetMode(userManager.MODE_DEFAULT)
		tgp.eventDetails(usrInfo, cb.From.ID, event, "", 0, false)
		tgp.updateEventUIAllPostings(event)
	}
	_, err := tgp.tg.Request(edit)
	if err != nil {
		log.Println(err)
	}
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setDate(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	editDate, ok := usrInfo.GetData(EDIT_EVENTDATE).(*time.Time)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setTimeFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	selDate, err := time.ParseInLocation(layoutISO, text, editDate.Location())
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Could not parse the date you provided. Please send it in the format YYYY-MM-DD."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	*editDate = changeJustDate(*editDate, selDate)

	// Save the changes to the string.
	err = setFunc(*editDate)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}

func changeJustDate(fullDate time.Time, newDate time.Time) time.Time {
	return time.Date(newDate.Year(), newDate.Month(), newDate.Day(), fullDate.Hour(), fullDate.Minute(), fullDate.Second(), 0, fullDate.Location())
}
func changeJustTime(fullDate time.Time, newDate time.Time) time.Time {
	return time.Date(fullDate.Year(), fullDate.Month(), fullDate.Day(), newDate.Hour(), newDate.Minute(), newDate.Second(), 0, fullDate.Location())
}

func (tgp *TGPlansBot) showAdvancedPanel(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, event dbInterface.DBEvent) {

	tgp.eventDetails(usrInfo, cb.From.ID, event, "", cb.Message.MessageID, true)

}

func (tgp *TGPlansBot) toggleItem(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, value bool, SetFunc setBoolFunc, event dbInterface.DBEvent, advanced bool) {

	// Save the changes
	err := SetFunc(!value)
	if err != nil {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf("error updating event: %v", err))
		_, _ = tgp.tg.Send(mObj)
		return
	}

	tgp.eventDetails(usrInfo, cb.From.ID, event, "", cb.Message.MessageID, advanced)
	tgp.updateEventUIAllPostings(event)
}

// directSetItem is used to directly set the value of an item
func (tgp *TGPlansBot) directSetItem(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery, value int, SetFunc setIntFunc, event dbInterface.DBEvent, advanced bool) {

	// Save the changes
	err := SetFunc(value)
	if err != nil {
		mObj := tgbotapi.NewMessage(cb.From.ID, usrInfo.Locale.Sprintf("error updating event: %v", err))
		_, _ = tgp.tg.Send(mObj)
		return
	}

	tgp.eventDetails(usrInfo, cb.From.ID, event, "", cb.Message.MessageID, advanced)
	tgp.updateEventUIAllPostings(event)
}

func (tgp *TGPlansBot) editChoiceItem(usrInfo *userManager.UserInfo, chatId int64, SetFunc setStringFunc, prompt string, choices []helpers.Tuple, advanced bool) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTCHOICES, choices)
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)
	usrInfo.SetData(EDIT_EVENT_RETURN_ADVANCED, advanced)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_CHOICE)

	// Build a keyboard of the choices
	var keyboard [][]tgbotapi.KeyboardButton
	for _, choice := range choices {
		keyboard = append(keyboard, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(choice.DisplayText),
		))
	}

	choiceKeyboard := tgbotapi.ReplyKeyboardMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		Keyboard:        keyboard,
	}

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = choiceKeyboard
	_, _ = tgp.tg.Send(mObj)
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setChoice(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setStringFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	advanced, ok := usrInfo.GetData(EDIT_EVENT_RETURN_ADVANCED).(bool)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	// Get the original list of choices
	choices, ok := usrInfo.GetData(EDIT_EVENTCHOICES).([]helpers.Tuple)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	// The text has to be one of the choices.
	found := ""
	for _, choice := range choices {
		if choice.DisplayText == text {
			found = choice.Key
			break
		}
	}

	if found == "" {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("The value you provided is not one of the choices."))
		return
	}

	err := setFunc(found)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, advanced)
	tgp.updateEventUIAllPostings(event)
}

// editStringItem puts them into a mode where they are editing a text item
func (tgp *TGPlansBot) editStringItem(usrInfo *userManager.UserInfo, chatId int64, EditItem string, SetFunc setStringFunc, prompt string, sendExisting bool) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_STRING)

	// Optionally this function can also send the existing value so the use can copy it easily.
	if sendExisting && EditItem != "" {
		mObj := tgbotapi.NewMessage(chatId, EditItem)
		mObj.ParseMode = ParseModeHtml
		mObj.LinkPreviewOptions.IsDisabled = true
		_, _ = tgp.tg.Send(mObj)
	}

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
	_, _ = tgp.tg.Send(mObj)
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setString(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setStringFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	htmlText := helpers.ConvertEntitiesToHTML(text, msg.Entities)

	// Set the string to this value.  This should update it in the struct.
	err := setFunc(htmlText)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}

// editPublicItem puts them into a mode where they are setting the public location
func (tgp *TGPlansBot) editPublicItem(usrInfo *userManager.UserInfo, chatId int64, SetFunc setPublicFunc, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to public location edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_PUBLIC)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
	_, _ = tgp.tg.Send(mObj)
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setPublic(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setPublicFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	if msg.Location == nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please send a Location via Telegram's 'Send Location' feature.  It should be near the event but doesn't have to be exact.  Check inside the 📎 menu."))
		return
	}

	// Set the string to this value.  This should update it in the struct.
	err := setFunc(true, msg.Location.Latitude, msg.Location.Longitude)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}

// editStringItem puts them into a mode where they are editing a text item
func (tgp *TGPlansBot) editPictureItem(usrInfo *userManager.UserInfo, chatId int64, EditItem string, SetFunc setStringFunc, prompt string, picPrompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to picture edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_PICTURE)

	// Send the current picture so we can see what it looks like
	if EditItem != "" {
		mObj := tgbotapi.NewMessage(chatId, picPrompt)
		mObj.LinkPreviewOptions.PreferLargeMedia = true
		mObj.LinkPreviewOptions.ShowAboveText = false
		mObj.LinkPreviewOptions.URL = EditItem
		_, _ = tgp.tg.Send(mObj)
	}

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
	_, _ = tgp.tg.Send(mObj)
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setPicture(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setStringFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	if len(msg.Photo) == 0 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please send a picture to go with your event."))
		return
	}

	// Download the file to our picture cache here on the server.
	url, err := tgp.saveLargestPhoto(event.ID(), msg.Photo)
	if err != nil {
		tgp.quickReply(msg, err.Error())
		return
	}

	// Set the string to this value.  This should update it in the struct.
	err = setFunc(url)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}

func (tgp *TGPlansBot) saveLargestPhoto(eventID uint, photos []tgbotapi.PhotoSize) (string, error) {

	largest := ""
	size := 0
	for _, photo := range photos {
		if photo.Height*photo.Width > size {
			size = photo.Height * photo.Width
			largest = photo.FileID
		}
	}

	return tgp.saveFile(eventID, largest)
}

// save this telegram file to disk in the cache
func (tgp *TGPlansBot) saveFile(eventID uint, fileID string) (string, error) {
	// generate a path for this file

	fURL, err := tgp.tg.GetFileDirectURL(fileID)
	if err != nil {
		return "", fmt.Errorf("error getting file: %v", err)
	}

	// Download the file data.
	// Use a hash for the filename so people can't guess the values
	name := fmt.Sprintf("%v.jpg", helpers.CalenFeedMD5(tgp.saltValue, int64(eventID)))
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, "html", "cache")
	err = os.MkdirAll(dir, 0777)

	fullpath := filepath.Join(dir, name)

	// write this file to disk here.
	resp, err := http.Get(fURL)
	if err != nil {
		return "", fmt.Errorf("error downloading file: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Writing to %v", fullpath)
	// Create the file
	out, err := os.Create(fullpath)
	if err != nil {
		return "", fmt.Errorf("error creating file %v: %v", fullpath, err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)

	// TODO: Remove hardcoded URL.
	return fmt.Sprintf("https://plansbot.avbrand.com/cache/%v", name), nil
}

// editNumberItem puts them into a mode where they are editing a number item
func (tgp *TGPlansBot) editNumberItem(usrInfo *userManager.UserInfo, chatId int64, EditItem int, SetFunc setIntFunc, prompt string) {
	// Store a pointer to the string we are trying to set.
	usrInfo.SetData(EDIT_EVENTNUMBER, EditItem)
	usrInfo.SetData(EDIT_EVENTSETFUNC, SetFunc)

	// Switch to string edit mode
	usrInfo.SetMode(userManager.MODE_EDIT_NUMBER)

	mObj := tgbotapi.NewMessage(chatId, prompt)
	mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false) // Remove the keyboard in case it is still kicking around
	_, _ = tgp.tg.Send(mObj)
}

// Called from when the mode above is finished
func (tgp *TGPlansBot) edit_setNumber(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// See if we are in a valid mode.
	event, ok := usrInfo.GetData(EDIT_EVENT).(dbInterface.DBEvent)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}
	setFunc, ok := usrInfo.GetData(EDIT_EVENTSETFUNC).(setIntFunc)
	if !ok {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf(GENERAL_ERROR))
		return
	}

	// Convert the value to a number
	num, err := strconv.Atoi(text)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Please provide a valid number"))
		return
	}

	err = setFunc(num)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error updating event: %v", err))
		return
	}

	// switch back to normal mode and display the event details
	usrInfo.SetMode(userManager.MODE_DEFAULT)
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, "", 0, false)
	tgp.updateEventUIAllPostings(event)
}
