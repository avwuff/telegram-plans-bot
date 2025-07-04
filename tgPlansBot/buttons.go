package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strconv"
	"strings"
	"time"
)

const layoutISO = "2006-01-02" // results in YYYY-MM-DD layout

// createCalendar will create a nice inline keyboard calendar.
func createCalendar(startDate time.Time, loc *localizer.Localizer, selDate time.Time) tgbotapi.InlineKeyboardMarkup {

	// Add a calendar to this message.
	var buttons [][]tgbotapi.InlineKeyboardButton
	// Add the rows of the calendar.

	dStart := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.Local)
	//dN := 0

	// Subtract days until we get to Sunday
	for {
		if dStart.Weekday() == time.Sunday {
			break
		}
		dStart = dStart.AddDate(0, 0, -1)
	}

	// ok, dStart should now be our first day in the calendar.
	// Start by listing the current month, and forward/back arrows
	row := make([]tgbotapi.InlineKeyboardButton, 3)
	row[0] = quickButton("◀", fmt.Sprintf("calen:month:%v", "P"))
	row[1] = quickButton(loc.FormatDate(startDate, "January 2006"), "calen:nothing")
	row[2] = quickButton("▶", fmt.Sprintf("calen:month:%v", "N"))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 7)
	row[0] = quickButton(loc.Sprintf("Su"), "calen:nothing")
	row[1] = quickButton(loc.Sprintf("Mo"), "calen:nothing")
	row[2] = quickButton(loc.Sprintf("Tu"), "calen:nothing")
	row[3] = quickButton(loc.Sprintf("We"), "calen:nothing")
	row[4] = quickButton(loc.Sprintf("Th"), "calen:nothing")
	row[5] = quickButton(loc.Sprintf("Fr"), "calen:nothing")
	row[6] = quickButton(loc.Sprintf("Sa"), "calen:nothing")
	buttons = append(buttons, row)

	// Date row buttons
	for {
		row = make([]tgbotapi.InlineKeyboardButton, 7)
		for i := 0; i < 7; i++ {
			row[i] = quickButton(fmt.Sprintf("%v", dStart.Day()), fmt.Sprintf("calen:sel:%v", dStart.Format(layoutISO)))
			// Is this the selected day?
			if selDate.Day() == dStart.Day() && selDate.Month() == dStart.Month() {
				row[i].Text = "[ " + row[i].Text + " ]"
			}

			// Add one to the day
			dStart = dStart.AddDate(0, 0, 1)
		}
		buttons = append(buttons, row)
		// once we have moved into the next month, it is time to quit
		if dStart.Month() != startDate.Month() {
			break
		}
	}

	// the CONTINUE button
	row = make([]tgbotapi.InlineKeyboardButton, 1)
	// January 2, 15:04:05, 2006
	row[0] = quickButton(loc.Sprintf("Continue with Date: %v", loc.FormatDate(selDate, "January 2")), "calen:finish")
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

}

func quickButton(Text, Callback string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.InlineKeyboardButton{
		Text:         Text,
		CallbackData: &Callback,
	}
}

func processDateClicks(selDate time.Time, cmd string) (outDate time.Time, finished bool) {
	// What did they click on?
	data := strings.Split(cmd, ":")
	if len(data) < 2 {
		return
	}

	switch data[1] { // command
	case "nothing":
		// do nothing
	case "month": // go to a different month

		switch data[2] {
		case "P": // previous month
			selDate = time.Date(selDate.Year(), selDate.Month(), 1, 0, 0, 0, 0, selDate.Location())
			selDate = selDate.AddDate(0, -1, 0)
		case "N": // next month
			selDate = time.Date(selDate.Year(), selDate.Month(), 1, 0, 0, 0, 0, selDate.Location())
			selDate = selDate.AddDate(0, 1, 0)
		}
		return selDate, false
	case "sel": // a day has been selected
		if len(data) < 3 {
			return
		}
		selDate, _ = time.ParseInLocation(layoutISO, data[2], selDate.Location())
		return selDate, false
	case "finish":
		return selDate, true
	}

	return selDate, false
}

func createTimeSelection(selTime time.Time, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {
	var buttons [][]tgbotapi.InlineKeyboardButton

	isPM := false
	realHour := selTime.Hour()
	if realHour >= 12 { // Use a 12-hour clock
		isPM = true
		realHour -= 12
	}

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	for i := 0; i < 12; i++ {
		t := fmt.Sprintf("%v", i)
		if i == 0 {
			t = "12"
		}
		n := i
		if isPM {
			n += 12
		}

		if i == realHour {
			t = fmt.Sprintf("[ %v ]", t)
		}

		row = append(row, quickButton(t, fmt.Sprintf("time:hour:%v", n)))

		// Telegram limits to 8 buttons per row
		if i == 5 {
			buttons = append(buttons, row)
			row = make([]tgbotapi.InlineKeyboardButton, 0)
		}
	}
	buttons = append(buttons, row)

	// Now do the minutes
	row = make([]tgbotapi.InlineKeyboardButton, 0)
	for i := 0; i < 60; i += 5 {
		t := fmt.Sprintf(":%02d", i) // leading zeros

		if i == selTime.Minute() {
			t = fmt.Sprintf("[ %v ]", t)
		}
		row = append(row, quickButton(t, fmt.Sprintf("time:minute:%v", i)))
		// Telegram limits to 8 buttons per row
		if i == 25 {
			buttons = append(buttons, row)
			row = make([]tgbotapi.InlineKeyboardButton, 0)
		}
	}
	buttons = append(buttons, row)

	// Add AM/PM buttons
	row = make([]tgbotapi.InlineKeyboardButton, 2)
	row[0] = quickButton(fmt.Sprintf("%v%v%v", iif(isPM, "", "[ "), "AM", iif(isPM, "", " ]")), fmt.Sprintf("time:hour:%v", iifint(isPM, selTime.Hour()-12, selTime.Hour())))
	row[1] = quickButton(fmt.Sprintf("%v%v%v", iif(!isPM, "", "[ "), "PM", iif(!isPM, "", " ]")), fmt.Sprintf("time:hour:%v", iifint(isPM, selTime.Hour(), selTime.Hour()+12)))
	buttons = append(buttons, row)

	// the CONTINUE button
	row = make([]tgbotapi.InlineKeyboardButton, 1)
	// January 2, 15:04:05, 2006
	row[0] = quickButton(loc.Sprintf("Continue with Time: %v", loc.FormatTimeForLocale(selTime)), "time:finish")
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func processTimeClicks(selTime time.Time, cmd string) (outTime time.Time, finished bool) {
	// What did they click on?
	data := strings.Split(cmd, ":")
	if len(data) < 2 {
		return
	}

	switch data[1] { // command
	case "nothing":
		// do nothing
	case "hour": // go to a different month
		if len(data) < 3 {
			return
		}
		h, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		selTime = time.Date(selTime.Year(), selTime.Month(), selTime.Day(), h, selTime.Minute(), 0, 0, selTime.Location())
		return selTime, false
	case "minute": // a day has been selected
		if len(data) < 3 {
			return
		}
		m, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		selTime = time.Date(selTime.Year(), selTime.Month(), selTime.Day(), selTime.Hour(), m, 0, 0, selTime.Location())
		return selTime, false
	case "finish":
		return selTime, true
	}
	return selTime, false
}

func iif(condition bool, trueText string, falseText string) string {
	if condition {
		return trueText
	}
	return falseText
}
func iifint(condition bool, trueText int, falseText int) string {
	if condition {
		return fmt.Sprintf("%v", trueText)
	}
	return fmt.Sprintf("%v", falseText)
}

// eventEditButtons creates the buttons that help you edit an event.
func eventEditButtons(event dbInterface.DBEvent, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 0)

	postButton := fmt.Sprintf("%v%v", POST_PREFIX, event.ID()) // Example: POST:1234
	row = append(row, tgbotapi.InlineKeyboardButton{
		Text:              loc.Sprintf("Share these plans in a chat ✅🔜"),
		SwitchInlineQuery: &postButton,
	})
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("🏆 Edit Name"), fmt.Sprintf("edit:%v:name", event.ID())))
	row = append(row, quickButton(loc.Sprintf("📆 Edit Date"), fmt.Sprintf("edit:%v:date", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("⏰ Edit Time"), fmt.Sprintf("edit:%v:time", event.ID())))
	row = append(row, quickButton(loc.Sprintf("📍 Edit Location"), fmt.Sprintf("edit:%v:location", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("🕴 Edit Hosted By"), fmt.Sprintf("edit:%v:hostedby", event.ID())))
	row = append(row, quickButton(loc.Sprintf("📝 Add Notes"), fmt.Sprintf("edit:%v:notes", event.ID())))
	buttons = append(buttons, row)

	isPublic, _, _ := event.Public()
	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("🖼 Add Picture"), fmt.Sprintf("edit:%v:picture", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	if isPublic {
		row = append(row, quickButton(loc.Sprintf("🔒 Remove from Directory"), fmt.Sprintf("edit:%v:notpublic", event.ID())))
	} else {
		row = append(row, quickButton(loc.Sprintf("🌎 List in Public Directory"), fmt.Sprintf("edit:%v:public", event.ID())))
	}
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("👫 Set Max Attendees"), fmt.Sprintf("edit:%v:maxattend", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💔 Allow Maybe: %v", iif(event.DisableMaybe(), loc.Sprintf("No"), loc.Sprintf("Yes"))), fmt.Sprintf("edit:%v:setmaybe", event.ID())))
	buttons = append(buttons, row)

	// If event is closed, show a button to reopen
	if event.Closed() {
		row = make([]tgbotapi.InlineKeyboardButton, 0)
		row = append(row, quickButton(loc.Sprintf("🏁 Event is closed. Reopen!"), fmt.Sprintf("edit:%v:reopen", event.ID())))
		buttons = append(buttons, row)
	}

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("📩 Allow Sharing: %v", iif(event.SharingAllowed(), loc.Sprintf("Yes"), loc.Sprintf("No"))), fmt.Sprintf("edit:%v:sharing", event.ID())))
	row = append(row, quickButton(loc.Sprintf("⚙ Advanced Options..."), fmt.Sprintf("edit:%v:advanced", event.ID())))
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// eventAdvancedButtons creates the buttons with extra options
func eventAdvancedButtons(event dbInterface.DBEvent, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("⚙ ADVANCED OPTIONS ⚙"), fmt.Sprintf("edit:%v:back", event.ID()))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("🐕 Suitwalk: %v", iif(event.Suitwalk(), loc.Sprintf("Yes"), loc.Sprintf("No"))), fmt.Sprintf("edit:%v:suitwalk", event.ID()))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("🙈 Hide Names: %v", iif(event.HideNames(), loc.Sprintf("Yes"), loc.Sprintf("No"))), fmt.Sprintf("edit:%v:hidenames", event.ID()))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 2)
	row[0] = quickButton(loc.Sprintf("🔠 Language"), fmt.Sprintf("edit:%v:language", event.ID()))
	row[1] = quickButton(loc.Sprintf("⌚ Time Zone"), fmt.Sprintf("edit:%v:timezone", event.ID()))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 2)
	row[0] = quickButton(loc.Sprintf("🗓 End Date"), fmt.Sprintf("edit:%v:enddate", event.ID()))
	row[1] = quickButton(loc.Sprintf("⌛ End Time"), fmt.Sprintf("edit:%v:endtime", event.ID()))
	buttons = append(buttons, row)

	// Limit the max number of guests people can bring
	row = make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("👨‍👩‍👦 Max Guests (+1's): %v", event.MaxGuests()), fmt.Sprintf("edit:%v:maxguests", event.ID()))
	buttons = append(buttons, row)

	if !event.Closed() { // Only show a Close button when the event isn't already closed.
		row = make([]tgbotapi.InlineKeyboardButton, 1)
		row[0] = quickButton(loc.Sprintf("❌ Close Event"), fmt.Sprintf("edit:%v:close", event.ID()))
		buttons = append(buttons, row)
	}

	row = make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("🔙 Back"), fmt.Sprintf("edit:%v:back", event.ID()))
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
