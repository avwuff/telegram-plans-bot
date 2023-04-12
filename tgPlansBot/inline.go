package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/helpers"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgWrapper"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"strconv"
	"strings"
)

const SHARE_PREFIX = "FPBSHARE-"
const POST_PREFIX = "POST:"

// handleInline comes from the user typing @furryplansbot followed by a query
// Generally this means we want to post the event in a chat.
func handleInline(tg *tgWrapper.Telegram, query *tgbotapi.InlineQuery) {

	// See what it is they want us to post.
	if query.Query != "" {

		var events []dbInterface.DBEvent

		// There are several ways the inline mode can be used.
		// Mode 1: Sharing from another chat.
		if strings.HasPrefix(query.Query, SHARE_PREFIX) {

			// Find a match for this one.
			hash := query.Query[len(SHARE_PREFIX):] // strip off the post prefix
			event, _, err := db.GetEventByHash(hash, saltValue, true)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			// Give the list here
			events = []dbInterface.DBEvent{event}

		} else if strings.HasPrefix(query.Query, POST_PREFIX) {
			// Find just this one event.
			eventId := query.Query[len(POST_PREFIX):] // strip off the post prefix
			id, err := strconv.Atoi(eventId)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			event, err := db.GetEvent(uint(id), query.From.ID)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			// Give the list here
			events = []dbInterface.DBEvent{event}
		} else {
			// Normal search by text
			var err error
			events, err = db.SearchEvents(query.From.ID, query.Query)
			if err != nil {
				answerWithList(tg, query, nil)
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
			article.InputMessageContent, article.ReplyMarkup = buildClickableStarter(event, loc)
			results = append(results, article)
		}

		answerWithList(tg, query, results)
	}
}

func buildClickableStarter(event dbInterface.DBEvent, loc *localizer.Localizer) (tgbotapi.InputTextMessageContent, *tgbotapi.InlineKeyboardMarkup) {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("👉 CLICK TO ACTIVATE EVENT 👈"), fmt.Sprintf("use:%v:activate", event.ID()))
	buttons = append(buttons, row)
	keyb := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	return tgbotapi.InputTextMessageContent{
		Text:                  fmt.Sprintf("%s\n\n%s", event.Name(), loc.Sprintf("Click the button below to activate this event.")),
		ParseMode:             tgWrapper.ParseModeHtml,
		DisableWebPagePreview: true,
	}, &keyb
}

func answerWithList(tg *tgWrapper.Telegram, query *tgbotapi.InlineQuery, results []interface{}) {

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		IsPersonal:    true,
		CacheTime:     1,
		Results:       results,
	}

	if _, err := tg.AnswerInlineQuery(inlineConf); err != nil {
		log.Println(err)
	}
}
