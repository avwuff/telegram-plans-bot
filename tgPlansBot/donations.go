package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
)

func (tgp *TGPlansBot) initDonateCommands() {
	// Guest commands
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_DONATE_CUSTOM, Handler: tgp.donateCustom})

	tgp.cmds.AddCB(tgCommands.Callback{DataPrefix: "donate", Handler: tgp.manage_clickDonate})
}

func (tgp *TGPlansBot) handleDonateStart(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	hash := text[len(DONATE_START_PREFIX):] // strip off the post prefix

	loc := usrInfo.Locale

	// Find this event by the hash
	event, _, err := tgp.db.GetEventByHash(hash, tgp.saltValue+GUEST_HASH_EXTRA, false)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Sorry, I wasn't able to find the event.  Try again."))
		return
	}

	// Does this event allow guests?
	if event.TotalCost() <= 0 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Sorry, but this event is not requesting donations."))
		return
	}

	// Switch them into Set Guest Names mode.
	//usrInfo.SetMode(userManager.MODE_GUESTS_SET_GUESTS)
	//usrInfo.SetData("GuestEvent", event.ID())
	//usrInfo.SetData("GuestList", []string{})

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("💰 5"), fmt.Sprintf("donate:%v:5", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 10"), fmt.Sprintf("donate:%v:10", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 15"), fmt.Sprintf("donate:%v:15", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("💰 20"), fmt.Sprintf("donate:%v:20", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 30"), fmt.Sprintf("donate:%v:30", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 50"), fmt.Sprintf("donate:%v:50", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("💰 100"), fmt.Sprintf("donate:%v:100", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 200"), fmt.Sprintf("donate:%v:200", event.ID())))
	row = append(row, quickButton(loc.Sprintf("💰 300"), fmt.Sprintf("donate:%v:300", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("💰 Custom Amount"), fmt.Sprintf("donate:%v:custom", event.ID())))
	buttons = append(buttons, row)

	row = make([]tgbotapi.InlineKeyboardButton, 0)
	row = append(row, quickButton(loc.Sprintf("❌ Remove my Donation"), fmt.Sprintf("donate:%v:0", event.ID())))
	buttons = append(buttons, row)

	buttonsMarkup := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	t := loc.Sprintf("Help recover the costs of %v! You can donate to the organizer with the following methods:\n\n%v\n\nOnce you have donated, come back here and click one of these buttons. If you have donated previously, indicate the total amount as this will replace your previous donation.", event.Name(), event.CostInfo())

	mObj := tgbotapi.NewMessage(msg.Chat.ID, t)
	mObj.ParseMode = ParseModeHtml
	mObj.ReplyMarkup = buttonsMarkup
	mObj.LinkPreviewOptions.IsDisabled = true
	_, err = tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}
}

// This handles one of the callback functions for when an 'edit' button is clicked.
func (tgp *TGPlansBot) manage_clickDonate(usrInfo *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {

	// Format is: //donate:<id>:donation amount
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

	name := helpers.HtmlEntities(cb.From.FirstName)

	// SIMPLE STRING EDIT
	switch data[2] {
	case "custom":
		usrInfo.SetMode(userManager.MODE_DONATE_CUSTOM)
		usrInfo.SetData("DonateEvent", event.ID())

		mObj := tgbotapi.NewMessage(cb.From.ID, loc.Sprintf("How much did you donate?"))
		mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
		_, err = tgp.tg.Send(mObj)
		if err != nil {
			log.Println(err)
		}

	default:
		// The number is a dollar amount.  Record it in the donations table.
		amount, _ := strconv.Atoi(data[2])
		event.RecordDonation(float64(amount), cb.From.ID, name)

		// Note that this phrase will get translated.
		mObj := tgbotapi.NewMessage(cb.From.ID, loc.Sprintf("Thank you for your donation!"))
		mObj.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
		_, err = tgp.tg.Send(mObj)
		if err != nil {
			log.Println(err)
		}

		// Also update the event in all the places
		tgp.updateEventUIAllPostings(event)
	}
}

// donateCustom is when a user makes a custom donation
func (tgp *TGPlansBot) donateCustom(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Get a reference to the event to make sure we don't go over the user limit
	eventId := usrInfo.GetData("DonateEvent").(uint)
	event, err := tgp.db.GetEvent(eventId, -1)
	if err != nil {
		return
	}

	// Convert the input text to a number.
	// Quickly remove currency symbols
	text = strings.ReplaceAll(text, "$", "")
	text = strings.ReplaceAll(text, "€", "")
	cost, err := strconv.ParseFloat(text, 64)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("I'm not sure what you're saying. Try sending just the number with no commas or currency symbols."))
		return

	}

	if cost < 0 || cost > 5000 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("That seems like a bad number."))
		return
	}

	name := helpers.HtmlEntities(msg.From.FirstName)
	event.RecordDonation(cost, msg.From.ID, name)

	tgp.quickReply(msg, usrInfo.Locale.Sprintf("Thank you for your donation!"))

	// Also update the event in all the places
	tgp.updateEventUIAllPostings(event)

}
