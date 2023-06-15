package tgPlansBot

import (
	"context"
	"fmt"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"time"
)

const limitToUser = int64(219073084)

var globalMsg string
var sendContext context.Context
var sendCancel context.CancelFunc

func (tgp *TGPlansBot) initGlobalMsgCommands() {
	tgp.cmds.Add(tgCommands.Command{Command: "/test", Handler: tgp.globalmsg_Test, Mode: userManager.MODE_GLOBALMSGSENDING})
	tgp.cmds.Add(tgCommands.Command{Command: "/send", Handler: tgp.globalmsg_Send, Mode: userManager.MODE_GLOBALMSGSENDING})
	tgp.cmds.Add(tgCommands.Command{Command: "/stop", Handler: tgp.globalmsg_Stop, Mode: userManager.MODE_GLOBALMSGSENDING})
	tgp.cmds.Add(tgCommands.Command{Mode: userManager.MODE_GLOBALMSG, Handler: tgp.globalmsg_Store})
}

// sendGlobalMessage allows a global message to be sent to the entire userbase of the Furry Plans Bot.
// Use with caution.  Generally only used by Av.
func (tgp *TGPlansBot) sendGlobalMessage(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Only let the authorized user use this function.
	if msg.Chat.ID != limitToUser {
		tgp.quickReply(msg, "Sorry, but this function is not available.")
		return
	}

	tgp.quickReply(msg, "Alright, we're sending a global message.  Go ahead with the message.")

	usrInfo.SetMode(userManager.MODE_GLOBALMSG)
}

func (tgp *TGPlansBot) globalmsg_Store(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {

	// Grab the message they sent.
	globalMsg = helpers.ConvertEntitiesToHTML(text, msg.Entities)

	usrInfo.SetMode(userManager.MODE_GLOBALMSGSENDING)
	tgp.quickReply(msg, "Got the message.\n\nTo test it with yourself, send /test.\nIf you're ready to send, say /send")

}

func (tgp *TGPlansBot) globalmsg_Test(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	// Just send the message to myself.
	tgp.quickReply(msg, "Here's what the message will look like:")

	// Send it to this one person.
	tgp.sendGlobalMsg(msg.Chat.ID)

	users, _ := tgp.db.GetAllUsers()
	tgp.quickReply(msg, fmt.Sprintf("Message will be sent to %v users when you are ready.", len(users)))
}

func (tgp *TGPlansBot) globalmsg_Stop(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	if sendContext != nil {
		sendCancel()
	}
	tgp.quickReply(msg, "Send cancelled.")
}

func (tgp *TGPlansBot) globalmsg_Send(usrInfo *userManager.UserInfo, msg *tgbotapi.Message, text string) {
	users, err := tgp.db.GetAllUsers()
	if err != nil {
		tgp.quickReply(msg, err.Error())
		return
	}

	mObj := tgbotapi.NewMessage(msg.Chat.ID, "Beginning send...\nSend /stop to stop.")
	mUpdate, err := tgp.tg.Send(mObj)
	if err != nil {
		log.Println(err)
	}

	// Set up a context that can be canceled.
	sendContext, sendCancel = context.WithCancel(context.Background())
	go tgp.startSend(msg.Chat.ID, mUpdate.MessageID, users)

	// start iterating through all the users.
	// limit to 20 a second.

}

func (tgp *TGPlansBot) startSend(chatId int64, msgId int, users []int64) {

	fail := 0

	for i, user := range users {
		if sendContext.Err() != nil {
			tgp.updateProgress(chatId, msgId, fmt.Sprintf("Cancelled.\n\nProcessed %v users with %v failures before stopping", i, fail))
			return
		}

		log.Println("Senging global message to: ", user)
		succ := tgp.sendGlobalMsg(user)
		if !succ {
			fail++
		}

		if i%50 == 0 {
			// update the message
			tgp.updateProgress(chatId, msgId, fmt.Sprintf("Progress: %v / %v\nSend /stop to stop.", i, len(users)))
		}

		time.Sleep(time.Millisecond + 100)
	}

	tgp.updateProgress(chatId, msgId, fmt.Sprintf("Sent to %v users with %v failures.", len(users), fail))
	sendCancel()
	sendContext = nil
}

func (tgp *TGPlansBot) updateProgress(chatId int64, msgId int, t string) {
	mObj := tgbotapi.NewEditMessageText(chatId, msgId, t)
	_, err := tgp.tg.Request(mObj)
	if err != nil {
		log.Println(err)
	}
}

func (tgp *TGPlansBot) sendGlobalMsg(chatId int64) bool {

	// Make sure they haven't gotten the message already.
	if !tgp.db.GlobalShouldSend(chatId) {
		return false
	}

	mObj := tgbotapi.NewMessage(chatId, globalMsg)
	mObj.ParseMode = ParseModeHtml
	mObj.DisableWebPagePreview = true

	_, err := tgp.tg.Send(mObj)
	if err != nil {
		// TODO: error if the user no longer exists.
		log.Println(err)
		tgp.db.GlobalMarkBadUser(chatId)
		return false
	}

	// Keep a record of who we have sent the message to already.
	tgp.db.GlobalSent(chatId)
	return true
}
