package tgPlansBot

import (
	"context"
	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"testing"
)

type sendUpdate struct {
	listenFunc func(tgbotapi.Update)
}

// helpers for setting up the test
func setupEnvironment(t *testing.T) (*dbInterface.DBFeaturesMock, *TelegramBotMock, *TGPlansBot, *sendUpdate) {
	// set up the languages
	localizer.InitLang()

	// create a mock database
	db := dbInterface.NewDBFeaturesMock(t)

	// set up the user manager
	userManager.Init(db)

	// create a mock bot
	tg := NewTelegramBotMock(t)

	myUpdater := &sendUpdate{}

	tg.On("SetMyCommands", mock.Anything).Return(nil, nil)
	tg.On("Listen", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		t.Log("got a listen func")
		myUpdater.listenFunc = args.Get(1).(func(tgbotapi.Update))
	})

	// Allow the user prefs to be retrieved
	// TODO: Allow these to be replaced.
	db.On("GetPrefs", mock.Anything).Return(dbInterface.Prefs{SetupComplete: true, Language: "en-US", TimeZone: "America/Los_Angeles"})

	// create a REAL plansbot that uses these mocks
	// this fires off a command to start listening, which we will catch with the mock framework.
	pb := &TGPlansBot{}
	pb.StartTG(context.Background(), "salt", db, tg)

	return db, tg, pb, myUpdater
}

func (sU *sendUpdate) sendText(text string) {
	sU.listenFunc(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 0,
			From: &tgbotapi.User{
				ID:        1234,
				FirstName: "Bob",
				LastName:  "Smith",
				UserName:  "bob",
			},
			SenderChat: nil,
			Date:       0,
			Chat: &tgbotapi.Chat{
				ID:        1234,
				UserName:  "bob",
				FirstName: "Bob",
				LastName:  "Smith",
			},
			Text: text,
		},
	})
}

func (sU *sendUpdate) sendButton(text string) {
	sU.listenFunc(tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID: "",
			From: &tgbotapi.User{
				ID:        1234,
				FirstName: "Bob",
				LastName:  "Smith",
				UserName:  "bob",
			},
			Message: &tgbotapi.Message{
				MessageID: 1111,
				Chat: &tgbotapi.Chat{
					ID:        1234,
					UserName:  "bob",
					FirstName: "Bob",
					LastName:  "Smith",
				},
			},
			InlineMessageID: "",
			ChatInstance:    "",
			Data:            text,
			GameShortName:   "",
		},
	})
}
