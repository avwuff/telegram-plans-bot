package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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
	// TODO: Month name from locale
	row[1] = quickButton(startDate.Format("January 2006"), "calen:nothing")
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
				// TODO A nicer way to mark this.
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
	row[0] = quickButton(loc.Sprintf("Continue with Selection: %v", selDate.Format("January 2")), "calen:finish")
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
	// TODO: Length check here
	data := strings.Split(cmd, ":")

	switch data[1] { // command
	case "nothing":
		// do nothing
	case "month": // go to a different month

		switch data[2] {
		case "P": // previous month
			selDate = time.Date(selDate.Year(), selDate.Month(), 1, 0, 0, 0, 0, time.Local)
			selDate = selDate.AddDate(0, -1, 0)
		case "N": // next month
			selDate = time.Date(selDate.Year(), selDate.Month(), 1, 0, 0, 0, 0, time.Local)
			selDate = selDate.AddDate(0, 1, 0)
		}
		return selDate, false
	case "sel": // a day has been selected
		selDate, _ = time.Parse(layoutISO, data[2])
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
	row[0] = quickButton(loc.Sprintf("Continue with Selection: %v", selTime.Format("15:04")), "time:finish")
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func processTimeClicks(selTime time.Time, cmd string) (outTime time.Time, finished bool) {
	// What did they click on?
	// TODO: Length check here
	data := strings.Split(cmd, ":")

	switch data[1] { // command
	case "nothing":
		// do nothing
	case "hour": // go to a different month
		h, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		selTime = time.Date(selTime.Year(), selTime.Month(), 1, h, selTime.Minute(), 0, 0, time.Local)
		return selTime, false
	case "minute": // a day has been selected
		m, err := strconv.Atoi(data[2])
		if err != nil {
			return
		}
		selTime = time.Date(selTime.Year(), selTime.Month(), 1, selTime.Hour(), m, 0, 0, time.Local)
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
func eventEditButtons(event *dbHelper.FurryPlans, loc *localizer.Localizer) tgbotapi.InlineKeyboardMarkup {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 0)

	postButton := fmt.Sprintf("POST:%v", event.EventID)
	row = append(row, tgbotapi.InlineKeyboardButton{
		Text:              loc.Sprintf("Share these plans in a chat ✅🔜"),
		SwitchInlineQuery: &postButton,
	})
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 3)
	row[0] = quickButton(loc.Sprintf("🏆 Edit Name"), fmt.Sprintf("edit:%v:name", event.EventID))
	row[1] = quickButton(loc.Sprintf("📆 Edit Date"), fmt.Sprintf("edit:%v:date", event.EventID))
	row[2] = quickButton(loc.Sprintf("⏰ Edit Time"), fmt.Sprintf("edit:%v:time", event.EventID))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 3)
	row[0] = quickButton(loc.Sprintf("📍 Edit Location"), fmt.Sprintf("edit:%v:location", event.EventID))
	row[1] = quickButton(loc.Sprintf("🕴 Edit Hosted By"), fmt.Sprintf("edit:%v:hostedby", event.EventID))
	row[2] = quickButton(loc.Sprintf("📝 Add Notes"), fmt.Sprintf("edit:%v:notes", event.EventID))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 2)
	row[0] = quickButton(loc.Sprintf("👫 Set Max Attendees"), fmt.Sprintf("edit:%v:maxattend", event.EventID))
	row[1] = quickButton(loc.Sprintf("💔 Allow Maybe: %v", iif(event.DisableMaybe == 1, loc.Sprintf("No"), loc.Sprintf("Yes"))), fmt.Sprintf("edit:%v:setmaybe", event.EventID))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("📩 Allow Sharing: %v", iif(event.AllowShare == 1, loc.Sprintf("Yes"), loc.Sprintf("No"))), fmt.Sprintf("edit:%v:sharing", event.EventID))
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
