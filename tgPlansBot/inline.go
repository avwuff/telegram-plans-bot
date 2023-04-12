package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
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

		var events []*dbHelper.FurryPlans

		// There are several ways the inline mode can be used.
		// Mode 1: Sharing from another chat.
		if strings.HasPrefix(query.Query, SHARE_PREFIX) {

			// Find a match for this one.
			hash := query.Query[len(SHARE_PREFIX):] // strip off the post prefix
			event, _, err := dbHelper.GetEventByHash(hash, saltValue)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			// Give the list here
			events = []*dbHelper.FurryPlans{event}

		} else if strings.HasPrefix(query.Query, POST_PREFIX) {
			// Find just this one event.
			eventId := query.Query[len(POST_PREFIX):] // strip off the post prefix
			id, err := strconv.Atoi(eventId)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			event, _, err := dbHelper.GetEvent(uint(id), query.From.ID)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			// Give the list here
			events = []*dbHelper.FurryPlans{event}
		} else {
			// Normal search by text
			var err error
			events, err = dbHelper.SearchEvents(query.From.ID, query.Query)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}
		}

		// If there's results, display them
		var results []interface{}
		for _, event := range events {

			// Use the locale of the event.
			loc := localizer.FromLanguage(event.Language)

			article := tgbotapi.NewInlineQueryResultArticle(
				fmt.Sprintf("%v%v", POST_PREFIX, event.EventID),
				fmt.Sprintf("%v - %v", helpers.StripHtmlRegex(event.Name), loc.FormatDateForLocale(event.DateTime.Time)),
				"")
			article.InputMessageContent, article.ReplyMarkup = buildClickableStarter(event, loc)
			results = append(results, article)
		}

		answerWithList(tg, query, results)
	}
}

func buildClickableStarter(event *dbHelper.FurryPlans, loc *localizer.Localizer) (tgbotapi.InputTextMessageContent, *tgbotapi.InlineKeyboardMarkup) {

	var buttons [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 1)
	row[0] = quickButton(loc.Sprintf("👉 CLICK TO ACTIVATE EVENT 👈"), fmt.Sprintf("use:%v:activate", event.EventID))
	buttons = append(buttons, row)
	keyb := tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}

	return tgbotapi.InputTextMessageContent{
		Text:                  fmt.Sprintf("%v\n\n%v", event.Name, loc.Sprintf("Click the button below to activate this event.")),
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
