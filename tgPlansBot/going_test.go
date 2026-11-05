package tgPlansBot

import (
	"errors"
	"testing"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_goingMoreInfo(t *testing.T) {
	localizer.InitLang()
	loc := localizer.FromLanguage("en-US")

	// Set up someone attending an event, who clicked on it in the list of their events.
	setup := func(t *testing.T) (*TGPlansBot, *TelegramBotMock, *userManager.UserInfo, *tgbotapi.CallbackQuery) {
		event := eventUIEvent(t, eventUIPeople(51, dbInterface.CANATTEND_YES, 1000)...)
		event.On("AmIAttending", int64(55)).Return(true)

		db := dbInterface.NewDBFeaturesMock(t)
		db.On("GetEvent", uint(1200), int64(-1)).Return(event, nil)

		tg := NewTelegramBotMock(t)
		cb := &tgbotapi.CallbackQuery{
			Data:    "moreinfo:1200",
			Message: &tgbotapi.Message{Chat: tgbotapi.Chat{ID: 55}},
		}
		return &TGPlansBot{tg: tg, db: db}, tg, &userManager.UserInfo{Locale: loc}, cb
	}

	t.Run("sent as rich message", func(t *testing.T) {
		pb, tg, usrInfo, cb := setup(t)

		var sent tgbotapi.SendRichMessageConfig
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			sent = args.Get(0).(tgbotapi.SendRichMessageConfig)
		}).Return(tgbotapi.Message{}, nil).Once()

		pb.goingMoreInfo(usrInfo, cb)

		assert.Equal(t, int64(55), sent.ChatID)
		assert.Contains(t, sent.RichMessage.HTML, "<details><summary><b>Attending: 51</b></summary>")
		assert.NotNil(t, sent.ReplyMarkup)
	})

	t.Run("rejected rich message falls back to classic", func(t *testing.T) {
		pb, tg, usrInfo, cb := setup(t)

		tg.On("Send", mock.AnythingOfType("tgbotapi.SendRichMessageConfig")).
			Return(tgbotapi.Message{}, errors.New("Bad Request: can't parse rich message")).Once()

		var classic tgbotapi.MessageConfig
		tg.On("Send", mock.AnythingOfType("tgbotapi.MessageConfig")).Run(func(args mock.Arguments) {
			classic = args.Get(0).(tgbotapi.MessageConfig)
		}).Return(tgbotapi.Message{}, nil).Once()

		pb.goingMoreInfo(usrInfo, cb)

		assert.Contains(t, classic.Text, "<b>Attending: 51</b>")
		assert.NotContains(t, classic.Text, "details")
		assert.Equal(t, ParseModeHtml, classic.ParseMode)
		require.NotNil(t, classic.ReplyMarkup)
	})

	t.Run("no fallback when rate limited", func(t *testing.T) {
		pb, tg, usrInfo, cb := setup(t)

		// The mock fails the test if there is a second send.
		tg.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("Too Many Requests: retry after 5")).Once()

		pb.goingMoreInfo(usrInfo, cb)
	})
}
