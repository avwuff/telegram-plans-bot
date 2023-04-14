package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"log"
	"strings"
	"time"
)

// This file handles the message queue if we get a retry_after error.

// inQueue: Is this message in the retry queue? If so, don't send it again.
func (tgp *TGPlansBot) inQueue(msgId string) bool {
	_, ok := tgp.queue.Load(msgId)
	return ok
}

func (tgp *TGPlansBot) addToQueue(event dbInterface.DBEvent, msgId string, after int) {
	// put it in the queue
	tgp.queue.Store(msgId, true)

	// start a timer to run this again
	time.AfterFunc(time.Second*time.Duration(after), func() {
		log.Printf("Timer elapsed, updating message %v now\n", msgId)
		loc := localizer.FromLanguage(event.Language())
		_, err := tgp.makeEventUI(0, event, loc, msgId)
		if err != nil {
			if strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
				// The message where this once was, was probably deleted.
				// So we delete the posting, so we don't try it again.
				event.DeletePosting(msgId)
			}
		}
		// now remove this one from the queue
		tgp.queue.Delete(msgId)
	})
}
