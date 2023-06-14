package tgPlansBot

import (
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strings"
	"time"
)

const (
	CREATE_NAME     = "CREATE_NAME"
	CREATE_DATE     = "CREATE_DATE"
	CREATE_TIME     = "CREATE_TIME"
	CREATE_LOCATION = "CREATE_LOCATION"

	CALEN_DATE_CHOOSE_TEXT = "Got it.  Choose a Date for the event by clicking on the Calendar below and then clicking Continue."
	CALEN_TIME_CHOOSE_TEXT = "Got it.  Choose a Time for the event by clicking on the times below and then clicking Continue."
	CHOOSE_LOCATION        = "Where does the event take place?  Specify the name or address as you might type into Google Maps."
	SET_NOTES              = `Specify any additional notes you'd like to show about the event.`
	SET_NOTES_2            = `If you don't have any notes, just click /skip.`
)

// create_SetName is after the user has responded with the name of the event.
func (tgp *TGPlansBot) create_SetName(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Initial date, set to NOW in the user's time zone.
	selDate := time.Now().In(usrInfo.TimeZone)

	// Store the name
	usrInfo.SetData(CREATE_NAME, helpers.ConvertEntitiesToHTML(text, msg.Entities))
	usrInfo.SetData(CREATE_DATE, selDate)

	usrInfo.SetMode(userManager.MODE_CREATE_EVENTDATE)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(CALEN_DATE_CHOOSE_TEXT))
	mObj.ReplyMarkup = createCalendar(selDate, usrInfo.Locale, selDate)
	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// DATE SELECTION
func (tgp *TGPlansBot) create_ClickDate(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which date element did they click?
	// We update the message as needed.

	edit := tgbotapi.NewEditMessageText(cb.From.ID, cb.Message.MessageID, CALEN_DATE_CHOOSE_TEXT)
	editDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		editDate = time.Now()
	}

	var finished bool
	editDate, finished = processDateClicks(editDate, cb.Data)
	usrInfo.SetData(CREATE_DATE, editDate)

	// Send the calendar again.
	if !finished {
		calen := createCalendar(editDate, usrInfo.Locale, editDate)
		edit.ReplyMarkup = &calen
	} else {
		// Move on to the next step.
		edit.Text = usrInfo.Locale.Sprintf("Date selected: %v", usrInfo.Locale.FormatDateForLocale(editDate))
		tgp.createSetDateAndContinue(usrInfo, cb.Message.Chat.ID, editDate)
	}

	_, err := tgp.tg.Request(edit)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) create_SetDate(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	selDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		selDate = time.Now()
	}
	// See if they spoke a date.
	selDate, err := time.ParseInLocation(layoutISO, text, selDate.Location())
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Could not parse the date you provided. Please send it in the format YYYY-MM-DD."))
		return
	}

	tgp.createSetDateAndContinue(usrInfo, msg.Chat.ID, selDate)
}

func (tgp *TGPlansBot) createSetDateAndContinue(usrInfo *userManager.UserInfo, chatId int64, selDate time.Time) {
	usrInfo.SetData(CREATE_DATE, selDate)
	usrInfo.SetData(CREATE_TIME, time.Now().In(usrInfo.TimeZone)) // only the TIME part is used.

	// Move to the time selection part.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTTIME)
	mObj := tgbotapi.NewMessage(chatId, usrInfo.Locale.Sprintf(CALEN_TIME_CHOOSE_TEXT))
	mObj.ReplyMarkup = createTimeSelection(time.Now(), usrInfo.Locale)
	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}

// TIME SELECTION
func (tgp *TGPlansBot) create_ClickTime(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which time element did they click?
	// We update the message as needed.

	edit := tgbotapi.NewEditMessageText(cb.From.ID, cb.Message.MessageID, CALEN_TIME_CHOOSE_TEXT)
	editTime, ok := usrInfo.GetData(CREATE_TIME).(time.Time)
	if !ok {
		editTime = time.Now()
	}

	var finished bool
	editTime, finished = processTimeClicks(editTime, cb.Data)
	usrInfo.SetData(CREATE_TIME, editTime)

	// Send the calendar again.
	if !finished {
		timeButtons := createTimeSelection(editTime, usrInfo.Locale)
		edit.ReplyMarkup = &timeButtons
	} else {
		// Move on to the next step.
		edit.Text = usrInfo.Locale.Sprintf("Time selected: %v", usrInfo.Locale.FormatTimeForLocale(editTime))
		tgp.createSetTimeAndContinue(usrInfo, cb.Message.Chat.ID, editTime)
	}

	_, err := tgp.tg.Request(edit)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) create_SetTime(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	selDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		selDate = time.Now()
	}
	// See if they spoke a time.
	selTime, err := time.ParseInLocation("15:04", text, selDate.Location())
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Could not parse the time you provided. Please send it in the format 22:03."))
		return
	}

	tgp.createSetTimeAndContinue(usrInfo, msg.Chat.ID, selTime)
}

func (tgp *TGPlansBot) createSetTimeAndContinue(usrInfo *userManager.UserInfo, chatId int64, selTime time.Time) {
	// Combine the date and time now
	selDate := usrInfo.GetData(CREATE_DATE).(time.Time)
	selDate = time.Date(selDate.Year(), selDate.Month(), selDate.Day(), selTime.Hour(), selTime.Minute(), 0, 0, selDate.Location())
	usrInfo.SetData(CREATE_DATE, selDate)

	// Move to the location selection part.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTLOCATION)
	mObj := tgbotapi.NewMessage(chatId, usrInfo.Locale.Sprintf(CHOOSE_LOCATION))
	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// LOCATION
func (tgp *TGPlansBot) create_SetLocation(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Store the location
	usrInfo.SetData(CREATE_LOCATION, helpers.ConvertEntitiesToHTML(text, msg.Entities))

	// Move to the location selection part.
	usrInfo.SetMode(userManager.MODE_CREATE_SETNOTES)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(SET_NOTES)+"\n\n"+usrInfo.Locale.Sprintf(SET_NOTES_2))
	_, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}

func (tgp *TGPlansBot) create_SetNotes(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Store the location
	selDate := usrInfo.GetData(CREATE_DATE).(time.Time)
	selName := usrInfo.GetData(CREATE_NAME).(string)
	selLoc := usrInfo.GetData(CREATE_LOCATION).(string)

	notes := helpers.ConvertEntitiesToHTML(text, msg.Entities)
	if notes == "/skip" {
		notes = ""
	}

	// Now finish the event
	eventId, err := tgp.createNewEvent(usrInfo, msg.Chat.ID, helpers.HtmlEntities(getOwnerName(msg.Chat)), selName, selDate, selLoc, notes)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("error creating event: %v", err))
		return
	}

	usrInfo.SetMode(userManager.MODE_DEFAULT)

	// Load the details about the event from the database.
	event, err := tgp.db.GetEvent(eventId, msg.Chat.ID)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Event not found"))
		return
	}
	tgp.eventDetails(usrInfo, msg.Chat.ID, event, usrInfo.Locale.Sprintf("Alright, I've created your event! You can now add additional content, or share it to another chat.\n\n"), 0, false)

}

func getOwnerName(chat *tgbotapi.Chat) string {
	if chat.UserName != "" {
		return chat.UserName
	}
	return strings.TrimSpace(chat.FirstName + " " + chat.LastName)
}

func (tgp *TGPlansBot) createNewEvent(usrInfo *userManager.UserInfo, chatId int64, ownerName string, name string, date time.Time, loc string, notes string) (uint, error) {
	// Add this event to the database.
	return tgp.db.CreateEvent(chatId, name, date, date.Location().String(), ownerName, loc, usrInfo.Prefs.Language, notes)
}
