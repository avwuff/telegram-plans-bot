package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

func (tgp *TGPlansBot) initGuestCommands() {
	// Guest commands
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_GUESTS_SET_GUESTS, Command: "/done", Handler: tgp.guestsDone})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_GUESTS_SET_GUESTS, Handler: tgp.addGuest})

}

func (tgp *TGPlansBot) handleGuestStart(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	hash := text[len(GUEST_START_PREFIX):] // strip off the post prefix

	// Find this event by the hash
	event, _, err := tgp.db.GetEventByHash(hash, tgp.saltValue+GUEST_HASH_EXTRA, false)
	if err != nil {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Sorry, I wasn't able to find the event.  Try again."))
		return
	}

	// Does this event allow guests?
	if event.MaxGuests() == 0 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Sorry, but this event does not allow you to bring any guests."))
		return
	}

	// Switch them into Set Guest Names mode.
	usrInfo.SetMode(userManager.MODE_GUESTS_SET_GUESTS)
	usrInfo.SetData("GuestEvent", event.ID())
	usrInfo.SetData("GuestList", []string{})

	tgp.quickReply(msg, usrInfo.Locale.Sprintf("Alright, tell me the name of each guest you are bringing to <b>%v</b>. You can bring up to %v.", event.Name(), event.MaxGuests()))
}

// addGuest is when the user wants to add a guest to their list.
func (tgp *TGPlansBot) addGuest(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Get a reference to the event to make sure we don't go over the user limit
	// TODO: User limit
	eventId := usrInfo.GetData("GuestEvent").(uint)
	event, err := tgp.db.GetEvent(eventId, -1)
	if err != nil {
		return
	}

	// Add this person to the list
	guests := usrInfo.GetData("GuestList").([]string)

	if len(text) > 32 {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Woah, slow down!  Just give me the name of the attendee."))
	}

	htmlText := helpers.ConvertEntitiesToHTML(text, msg.Entities)

	// Strip HTML off the text so that guests can't include junk
	htmlText = helpers.HtmlEntities(helpers.StripHtmlRegex(htmlText))
	// Also remove newlines
	htmlText = strings.ReplaceAll(htmlText, "\n", "")

	guests = append(guests, htmlText)
	usrInfo.SetData("GuestList", guests)

	if len(guests) >= event.MaxGuests() {
		tgp.guestsDone(usrInfo, msg, "")
		return
	}

	tgp.quickReply(msg, usrInfo.Locale.Sprintf("Got it.  Who's the next guest?  If that's the last one, click /done."))
}

func (tgp *TGPlansBot) guestsDone(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	defer usrInfo.SetMode(userManager.MODE_DEFAULT)

	eventId := usrInfo.GetData("GuestEvent").(uint)
	event, err := tgp.db.GetEvent(eventId, -1)
	if err != nil {
		return
	}

	guests := usrInfo.GetData("GuestList").([]string)
	name := helpers.HtmlEntities(msg.From.FirstName)
	reply := event.Attending(msg.From.ID, name, dbInterface.CANATTEND_YES, len(guests), guests)

	if reply == dbInterface.ATTEND_FULL {
		tgp.quickReply(msg, usrInfo.Locale.Sprintf("Sorry, this event is currently full!"))
		return
	}

	tgp.quickReply(msg, usrInfo.Locale.Sprintf("You're all set!  I've added those people as your guests at <b>%v</b>. You can now return to the chat you were in previously.\n\nThanks for using the Furry Plans Bot!", event.Name()))

	// Also update the event in all the places
	tgp.updateEventUIAllPostings(event)
}
