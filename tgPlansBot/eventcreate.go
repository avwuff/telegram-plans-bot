package tgPlansBot

import (
	"database/sql"
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
	"time"
)

const (
	CREATE_NAME = "CREATE_NAME"
	CREATE_DATE = "CREATE_DATE"
	CREATE_TIME = "CREATE_TIME"

	CALEN_DATE_CHOOSE_TEXT = "Got it.  Choose a Date for the event by clicking on the Calendar below and then clicking Continue."
	CALEN_TIME_CHOOSE_TEXT = "Got it.  Choose a Time for the event by clicking on the times below and then clicking Continue."
	CHOOSE_LOCATION        = "Where does the event take place?  Specify the name or address as you might type into Google Maps."
)

// create_SetName is after the user has responded with the name of the event.
func create_SetName(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Initial date, set to NOW in the user's time zone.
	selDate := time.Now().In(usrInfo.TimeZone)

	// Store the name
	usrInfo.SetData(CREATE_NAME, tg.ConvertEntitiesToHTML(text, msg.Entities))
	usrInfo.SetData(CREATE_DATE, selDate)

	usrInfo.SetMode(userManager.MODE_CREATE_EVENTDATE)
	mObj := tgbotapi.NewMessage(msg.Chat.ID, usrInfo.Locale.Sprintf(CALEN_DATE_CHOOSE_TEXT))
	mObj.ReplyMarkup = createCalendar(selDate, usrInfo.Locale, selDate)
	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// DATE SELECTION
func create_ClickDate(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which date element did they click?
	// We update the message as needed.

	edit := tgbotapi.NewEditMessageText(int64(cb.From.ID), cb.Message.MessageID, CALEN_DATE_CHOOSE_TEXT)
	selDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		selDate = time.Now()
	}

	var finished bool
	selDate, finished = processDateClicks(selDate, cb.Data)
	usrInfo.SetData(CREATE_DATE, selDate)

	// Send the calendar again.
	if !finished {
		calen := createCalendar(selDate, usrInfo.Locale, selDate)
		edit.ReplyMarkup = &calen
	} else {
		// Move on to the next step.
		edit.Text = usrInfo.Locale.Sprintf("Date selected: %v", selDate.Format("January 2, 2006"))
		createSetDateAndContinue(tg, usrInfo, cb.Message.Chat.ID, selDate)
	}

	_, err := tg.Send(edit)
	if err != nil {
		log.Println(err)
	}
}

func create_SetDate(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	selDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		selDate = time.Now()
	}
	// See if they spoke a date.
	selDate, err := time.ParseInLocation(layoutISO, text, selDate.Location())
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Could not parse the date you provided. Please send it in the format YYYY-MM-DD."))
		return
	}

	createSetDateAndContinue(tg, usrInfo, msg.Chat.ID, selDate)
}

func createSetDateAndContinue(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, selDate time.Time) {
	usrInfo.SetData(CREATE_DATE, selDate)
	usrInfo.SetData(CREATE_TIME, time.Now().In(usrInfo.TimeZone)) // only the TIME part is used.

	// Move to the time selection part.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTTIME)
	mObj := tgbotapi.NewMessage(chatId, usrInfo.Locale.Sprintf(CALEN_TIME_CHOOSE_TEXT))
	mObj.ReplyMarkup = createTimeSelection(time.Now(), usrInfo.Locale)
	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

}

// TIME SELECTION
func create_ClickTime(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
	// Ok, which time element did they click?
	// We update the message as needed.

	edit := tgbotapi.NewEditMessageText(int64(cb.From.ID), cb.Message.MessageID, CALEN_TIME_CHOOSE_TEXT)
	selTime, ok := usrInfo.GetData(CREATE_TIME).(time.Time)
	if !ok {
		selTime = time.Now()
	}

	var finished bool
	selTime, finished = processTimeClicks(selTime, cb.Data)
	usrInfo.SetData(CREATE_TIME, selTime)

	// Send the calendar again.
	if !finished {
		timeButtons := createTimeSelection(selTime, usrInfo.Locale)
		edit.ReplyMarkup = &timeButtons
	} else {
		// Move on to the next step.
		edit.Text = usrInfo.Locale.Sprintf("Time selected: %v", selTime.Format("15:04")) // TODO switch to AM/PM
		createSetTimeAndContinue(tg, usrInfo, cb.Message.Chat.ID, selTime)
	}

	_, err := tg.Send(edit)
	if err != nil {
		log.Println(err)
	}
}

func create_SetTime(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	selDate, ok := usrInfo.GetData(CREATE_DATE).(time.Time)
	if !ok {
		selDate = time.Now()
	}
	// See if they spoke a time.
	selTime, err := time.ParseInLocation("15:04", text, selDate.Location())
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("Could not parse the time you provided. Please send it in the format 22:03."))
		return
	}

	createSetTimeAndContinue(tg, usrInfo, msg.Chat.ID, selTime)
}

func createSetTimeAndContinue(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, selTime time.Time) {
	// Combine the date and time now
	selDate := usrInfo.GetData(CREATE_DATE).(time.Time)
	selDate = time.Date(selDate.Year(), selDate.Month(), selDate.Day(), selTime.Hour(), selTime.Minute(), 0, 0, selDate.Location())
	usrInfo.SetData(CREATE_DATE, selDate)

	// Move to the time selection part.
	usrInfo.SetMode(userManager.MODE_CREATE_EVENTLOCATION)
	mObj := tgbotapi.NewMessage(chatId, usrInfo.Locale.Sprintf(CHOOSE_LOCATION))
	_, err := tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// LOCATION
func create_SetLocation(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Store the location
	selDate := usrInfo.GetData(CREATE_DATE).(time.Time)
	selName := usrInfo.GetData(CREATE_NAME).(string)

	// Now finish the event
	eventId, err := createNewEvent(tg, usrInfo, msg.Chat.ID, getOwnerName(msg.Chat), selName, selDate, tg.ConvertEntitiesToHTML(text, msg.Entities))
	if err != nil {
		quickReply(tg, msg, usrInfo.Locale.Sprintf("error creating event: %v", err))
		return
	}

	usrInfo.SetMode(userManager.MODE_DEFAULT)
	eventDetails(tg, usrInfo, msg.Chat.ID, eventId, usrInfo.Locale.Sprintf("Alright, I've created your event! You can now add additional content, or share it to another chat.\n\n"), 0, false)
}

func getOwnerName(chat *tgbotapi.Chat) string {
	if chat.UserName != "" {
		return chat.UserName
	}
	return strings.TrimSpace(chat.FirstName + " " + chat.LastName)
}

func createNewEvent(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, chatId int64, ownerName string, name string, date time.Time, loc string) (uint, error) {
	// Add this event to the database.
	event := dbHelper.FurryPlans{
		OwnerID:   fmt.Sprintf("%v", chatId),
		Name:      name,
		DateTime:  sql.NullTime{Time: date, Valid: true},
		TimeZone:  date.Location().String(),
		CreatedAt: sql.NullTime{Time: time.Now(), Valid: true},
		OwnerName: ownerName,
		Location:  loc,
	}
	return dbHelper.CreateEvent(&event)
}
