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

func Test_notGoingHandler(t *testing.T) {
	localizer.InitLang()
	usrInfo := &userManager.UserInfo{Locale: localizer.FromLanguage("en-US")}
	msg := &tgbotapi.Message{Chat: tgbotapi.Chat{ID: 55}}

	t.Run("lists declined events", func(t *testing.T) {
		event := eventUIEvent(t)
		event.On("GetCanAttend").Return(dbInterface.CANATTEND_NO)

		db := dbInterface.NewDBFeaturesMock(t)
		db.On("NotGoingEvents", int64(55)).Return([]dbInterface.DBEvent{event}, nil)
		tg := NewTelegramBotMock(t)

		var sent tgbotapi.MessageConfig
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			sent = args.Get(0).(tgbotapi.MessageConfig)
		}).Return(tgbotapi.Message{}, nil).Once()

		pb := &TGPlansBot{tg: tg, db: db}
		pb.notGoingHandler(usrInfo, msg, "")

		assert.Contains(t, sent.Text, "you can't make it")
		markup, ok := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
		require.True(t, ok)
		require.Len(t, markup.InlineKeyboard, 1)
		button := markup.InlineKeyboard[0][0]
		assert.Contains(t, button.Text, "❌Test Event")
		require.NotNil(t, button.CallbackData)
		assert.Equal(t, "moreinfo:1200", *button.CallbackData)
	})

	t.Run("says so when there are none", func(t *testing.T) {
		db := dbInterface.NewDBFeaturesMock(t)
		db.On("NotGoingEvents", int64(55)).Return([]dbInterface.DBEvent{}, nil)
		tg := NewTelegramBotMock(t)

		var sent tgbotapi.MessageConfig
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			sent = args.Get(0).(tgbotapi.MessageConfig)
		}).Return(tgbotapi.Message{}, nil).Once()

		pb := &TGPlansBot{tg: tg, db: db}
		pb.notGoingHandler(usrInfo, msg, "")

		assert.Contains(t, sent.Text, "You haven't said")
		_, isKeyboard := sent.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
		assert.False(t, isKeyboard)
	})
}

// Clicking an event in /notgoing has to open it, even when private, or the person couldn't change their mind.
// Someone who never responded to a private event must still not be able to see it.
func Test_goingMoreInfo_privateEvent(t *testing.T) {
	localizer.InitLang()
	usrInfo := &userManager.UserInfo{Locale: localizer.FromLanguage("en-US")}
	cb := &tgbotapi.CallbackQuery{
		Data:    "moreinfo:1200",
		Message: &tgbotapi.Message{Chat: tgbotapi.Chat{ID: 55}},
	}

	setup := func(t *testing.T, mine []*dbInterface.Attend) (*TGPlansBot, *TelegramBotMock) {
		event := eventUIEvent(t)
		event.On("AmIAttending", int64(55)).Return(false)
		event.On("GetAttending", int64(55)).Return(mine, nil)
		event.On("Public").Return(false, 0.0, 0.0).Maybe()

		db := dbInterface.NewDBFeaturesMock(t)
		db.On("GetEvent", uint(1200), int64(-1)).Return(event, nil)
		tg := NewTelegramBotMock(t)
		return &TGPlansBot{tg: tg, db: db}, tg
	}

	t.Run("shown to someone who declined", func(t *testing.T) {
		pb, tg := setup(t, []*dbInterface.Attend{{UserID: 55, CanAttend: int(dbInterface.CANATTEND_NO)}})
		tg.On("Send", mock.AnythingOfType("tgbotapi.SendRichMessageConfig")).Return(tgbotapi.Message{}, nil).Once()

		pb.goingMoreInfo(usrInfo, cb)
	})

	t.Run("refused to someone who never responded", func(t *testing.T) {
		pb, tg := setup(t, nil)
		var sent tgbotapi.MessageConfig
		tg.On("Send", mock.AnythingOfType("tgbotapi.MessageConfig")).Run(func(args mock.Arguments) {
			sent = args.Get(0).(tgbotapi.MessageConfig)
		}).Return(tgbotapi.Message{}, nil).Once()

		pb.goingMoreInfo(usrInfo, cb)

		assert.Equal(t, "Not an event you are attending.", sent.Text)
	})
}

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
