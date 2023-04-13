package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgWrapper"
	"log"
	"strings"
	"sync"
	"time"
)

// This file handles the message queue if we get a retry_after error.

var queue sync.Map

// inQueue: Is this message in the retry queue? If so, don't send it again.
func inQueue(msgId string) bool {
	_, ok := queue.Load(msgId)
	return ok
}

func addToQueue(tg *tgWrapper.Telegram, event dbInterface.DBEvent, msgId string, after int) {
	// put it in the queue
	queue.Store(msgId, true)

	// start a timer to run this again
	time.AfterFunc(time.Second*time.Duration(after), func() {
		log.Printf("Timer elapsed, updating message %v now\n", msgId)
		loc := localizer.FromLanguage(event.Language())
		_, err := makeEventUI(tg, 0, event, loc, msgId)
		if err != nil {
			if strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
				// The message where this once was, was probably deleted.
				// So we delete the posting, so we don't try it again.
				event.DeletePosting(msgId)
			}
		}
		// now remove this one from the queue
		queue.Delete(msgId)
	})
}
