package tgPlansBot

import (
	"fmt"
	"furryplansbot.avbrand.com/dbHelper"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgWrapper"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"html"
	"log"
	"regexp"
	"strconv"
	"strings"
)

const SHARE_PREFIX = "FPBSHARE-"
const POST_PREFIX = "POST:"

// handleInline comes from the user typing @furryplansbot followed by a query
// Generally this means we want to post the event in a chat.
func handleInline(tg *tgWrapper.Telegram, query *tgbotapi.InlineQuery) {

	// Note that the request may not be coming from a user that has ever used the bot.
	usrInfo := userManager.Get(int64(query.From.ID))

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

			event, _, err := dbHelper.GetEvent(uint(id), int64(query.From.ID))
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}

			// Give the list here
			events = []*dbHelper.FurryPlans{event}
		} else {
			// Normal search by text
			var err error
			events, err = dbHelper.SearchEvents(int64(query.From.ID), query.Query)
			if err != nil {
				answerWithList(tg, query, nil)
				return
			}
		}

		// If there's results, display them
		var results []interface{}
		for _, event := range events {

			article := tgbotapi.NewInlineQueryResultArticle(
				fmt.Sprintf("%v%v", POST_PREFIX, event.EventID),
				fmt.Sprintf("%v - %v", stripHtmlRegex(event.Name), event.DateTime.Time.Format(layoutISO)), // TODO Proper time format
				"")
			article.InputMessageContent, article.ReplyMarkup = buildClickableStarter(event, usrInfo.Locale)
			results = append(results, article)
		}

		answerWithList(tg, query, results)
	}
}

// This method uses a regular expresion to remove HTML tags.
func stripHtmlRegex(s string) string {
	r := regexp.MustCompile(`<.*?>`)
	return html.UnescapeString(r.ReplaceAllString(s, ""))
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
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	}, &keyb
}

func answerWithList(tg *tgWrapper.Telegram, query *tgbotapi.InlineQuery, results []interface{}) {

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       results,
	}

	if _, err := tg.AnswerInlineQuery(inlineConf); err != nil {
		log.Println(err)
	}
}
