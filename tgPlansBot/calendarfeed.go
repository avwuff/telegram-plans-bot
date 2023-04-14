package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// calendarFeed provides a calendar feed URL that can provide a feed of events.
func (tgp *TGPlansBot) calendarFeed(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// TODO: This should not be a hard-coded URL
	ICalURL := fmt.Sprintf("https://plansbot.avbrand.com/feed/%v/%v/plans.ics", msg.Chat.ID, helpers.CalenFeedMD5(tgp.saltValue, msg.Chat.ID))
	tgp.quickReply(msg, usrInfo.Locale.Sprintf(`Cool, here is an iCal feed of all the events you've said 'Yes' or 'Maybe' to:

%v

You can add this feed URL to your Google Calendar or Outlook, and any events you've said 'Yes' or 'Maybe' to will appear in your Calendar automatically, and stay up to date!`, ICalURL))
}
