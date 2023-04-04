package tgPlansBot

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// calendarFeed provides a calendar feed URL that can provide a feed of events.
// Eventually the code for this will be moved into this bot directly.
func calendarFeed(tg *tgWrapper.Telegram, usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	ICalURL := fmt.Sprintf("http://www.avbrand.com/telegram/plansbot/%v/%v/furryplans.ics", msg.Chat.ID, calenFeedMD5(msg.Chat.ID))
	quickReply(tg, msg, usrInfo.Locale.Sprintf("Cool, here is an iCal feed of all the events you've said 'Yes' or 'Maybe' to:\n\n"+ICalURL+"\n\nYou can add this feed URL to your Google Calendar or Outlook, and any events you've said 'Yes' or 'Maybe' to will appear in your Calendar automatically, and stay up to date!"))
}

func calenFeedMD5(id int64) string {
	str := fmt.Sprintf("%v%v", id, saltValue)
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}
