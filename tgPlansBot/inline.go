package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
)

const SHARE_PREFIX = "FPBSHARE-"
const POST_PREFIX = "POST:"
const GUESTS_PREFIX = "GUESTS:"
const DONATE_PREFIX = "DONATE:"
const GUEST_HASH_EXTRA = "2oi3mi2o" // Add some extra crap to the hash so that the guest hash doesn't match the main hash
const GUEST_START_PREFIX = "SetGuestNames_"
const DONATE_START_PREFIX = "Donate_"

// handleInline comes from the user typing @furryplansbot followed by a query
// Generally this means we want to post the event in a chat.
func (tgp *TGPlansBot) handleInline(query *tgbotapi.InlineQuery) {

	// See what it is they want us to post.
	if query.Query != "" {

		var events []dbInterface.DBEvent

		// There are several ways the inline mode can be used.

		if strings.HasPrefix(query.Query, DONATE_PREFIX) { // Donations
			hash := query.Query[len(DONATE_PREFIX):] // strip off the post prefix
			event, _, err := tgp.db.GetEventByHash(hash, tgp.saltValue+GUEST_HASH_EXTRA, false)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}

			loc := localizer.FromLanguage(event.Language())

			button := &tgbotapi.InlineQueryResultsButton{
				Text:       loc.Sprintf("Click here for donation information..."),
				WebApp:     nil,
				StartParam: DONATE_START_PREFIX + hash,
			}

			tgp.answerWithList(query, nil, button)

		} else if strings.HasPrefix(query.Query, GUESTS_PREFIX) { // Specifying guests
			hash := query.Query[len(GUESTS_PREFIX):] // strip off the post prefix
			event, _, err := tgp.db.GetEventByHash(hash, tgp.saltValue+GUEST_HASH_EXTRA, false)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}

			loc := localizer.FromLanguage(event.Language())

			button := &tgbotapi.InlineQueryResultsButton{
				Text:       loc.Sprintf("Click here to specify Guest names..."),
				WebApp:     nil,
				StartParam: GUEST_START_PREFIX + hash,
			}

			tgp.answerWithList(query, nil, button)

		} else if strings.HasPrefix(query.Query, SHARE_PREFIX) { // Mode 1: Sharing from another chat.

			// Find a match for this one.
			hash := query.Query[len(SHARE_PREFIX):] // strip off the post prefix
			event, _, err := tgp.db.GetEventByHash(hash, tgp.saltValue, true)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}

			// Give the list here
			events = []dbInterface.DBEvent{event}

		} else if strings.HasPrefix(query.Query, POST_PREFIX) {
			// Find just this one event.
			eventId := query.Query[len(POST_PREFIX):] // strip off the post prefix
			id, err := strconv.Atoi(eventId)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}

			event, err := tgp.db.GetEvent(uint(id), query.From.ID)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}

			// Give the list here
			events = []dbInterface.DBEvent{event}
		} else {
			// Normal search by text
			var err error
			events, err = tgp.db.SearchEvents(query.From.ID, query.Query)
			if err != nil {
				tgp.answerWithList(query, nil, nil)
				return
			}
		}

		// If there's results, display them
		var results []interface{}
		for _, event := range events {

			// Use the locale of the event.
			loc := localizer.FromLanguage(event.Language())

			article := tgbotapi.NewInlineQueryResultArticle(
				fmt.Sprintf("%v%v", POST_PREFIX, event.ID()),
				fmt.Sprintf("%v - %v", helpers.StripHtmlRegex(event.Name()), loc.FormatDateForLocale(event.DateTime())),
				"")
			article.InputMessageContent, article.ReplyMarkup = tgp.buildClickableStarter(event, loc)
			results = append(results, article)
		}

		tgp.answerWithList(query, results, nil)
	}
}

func (tgp *TGPlansBot) buildClickableStarter(event dbInterface.DBEvent, loc *localizer.Localizer) (tgbotapi.InputTextMessageContent, *tgbotapi.InlineKeyboardMarkup) {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("👉 CLICK TO ACTIVATE EVENT 👈"), fmt.Sprintf("use:%v:activate", event.ID()))
	buttons = append(buttons, row)
	keyb := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	return tgbotapi.InputTextMessageContent{
		Text:      fmt.Sprintf("%s\n\n%s", event.Name(), loc.Sprintf("Click the button below to activate this event.")),
		ParseMode: ParseModeHtml,
		LinkPreviewOptions: &tgbotapi.LinkPreviewOptions{
			IsDisabled: true,
		},
	}, &keyb
}

func (tgp *TGPlansBot) answerWithList(query *tgbotapi.InlineQuery, results []interface{}, button *tgbotapi.InlineQueryResultsButton) {

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		IsPersonal:    true,
		CacheTime:     1,
		Results:       results,
		Button:        button,
	}

	if _, err := tgp.tg.AnswerInlineQuery(inlineConf); err != nil {
		log.Println(err)
	}
}
