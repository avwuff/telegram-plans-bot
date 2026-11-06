package tgPlansBot

import (
	"testing"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	"furryplansbot.avbrand.com/tgCommands"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_attendChoice(t *testing.T) {
	tests := []struct {
		value     string
		suitwalk  bool
		maxGuests int
		wantOk    bool
		want      dbInterface.CanAttend
		people    int
	}{
		{value: "0", maxGuests: 0, wantOk: true, want: dbInterface.CANATTEND_YES},
		{value: "3", maxGuests: 3, wantOk: true, want: dbInterface.CANATTEND_YES, people: 3},
		{value: "1", maxGuests: 0},
		{value: "9", maxGuests: 2},
		{value: "-1", maxGuests: 2},
		{value: "-1000", maxGuests: 2},
		{value: "abc", maxGuests: 2},
		{value: "", maxGuests: 2},
		// Suiting and photographer are only for suitwalks.
		{value: "20", maxGuests: 2},
		{value: "30", maxGuests: 2},
		// Suitwalks always offer up to two guests.
		{value: "2", suitwalk: true, wantOk: true, want: dbInterface.CANATTEND_YES, people: 2},
		{value: "20", suitwalk: true, wantOk: true, want: dbInterface.CANATTEND_SUITING},
		{value: "22", suitwalk: true, wantOk: true, want: dbInterface.CANATTEND_SUITING, people: 2},
		{value: "30", suitwalk: true, wantOk: true, want: dbInterface.CANATTEND_PHOTOGRAPHER},
		{value: "32", suitwalk: true, wantOk: true, want: dbInterface.CANATTEND_PHOTOGRAPHER, people: 2},
		{value: "3", suitwalk: true},
		{value: "23", suitwalk: true},
		{value: "33", suitwalk: true},
		{value: "-20", suitwalk: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			event := dbInterface.NewDBEventMock(t)
			event.On("Suitwalk").Return(tt.suitwalk).Maybe()
			event.On("MaxGuests").Return(tt.maxGuests).Maybe()

			canAtt, people, ok := attendChoice(event, tt.value)

			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.want, canAtt)
				assert.Equal(t, tt.people, people)
			}
		})
	}
}

// forgedSetup is someone clicking a button with data they made up. The event mock has no 'Attending',
// so it would panic if someone was added to the event.
func forgedSetup(t *testing.T, data string) (*TGPlansBot, *userManager.UserInfo, *tgbotapi.CallbackQuery) {
	localizer.InitLang()

	event := dbInterface.NewDBEventMock(t)
	event.On("Language").Return("en-US").Maybe()
	event.On("Closed").Return(false).Maybe()
	event.On("Suitwalk").Return(false).Maybe()
	event.On("MaxGuests").Return(2).Maybe()
	event.On("SavePostingRegular", mock.Anything, mock.Anything).Return(nil).Maybe()

	db := dbInterface.NewDBFeaturesMock(t)
	db.On("GetEvent", uint(5), int64(-1)).Return(event, nil).Maybe()

	tg := NewTelegramBotMock(t)
	tg.On("AnswerCallbackQuery", mock.Anything).Return(&tgbotapi.APIResponse{Ok: true}, nil).Maybe()

	cb := &tgbotapi.CallbackQuery{
		Data:    data,
		From:    &tgbotapi.User{ID: 999, FirstName: "someone"},
		Message: &tgbotapi.Message{MessageID: 77, Chat: tgbotapi.Chat{ID: 999}},
	}
	return &TGPlansBot{db: db, tg: tg}, &userManager.UserInfo{Locale: localizer.FromLanguage("en-US")}, cb
}

func Test_ui_Attending_forgedData(t *testing.T) {
	for _, data := range []string{"use", "attending", "attending:", "attending:abc:0", "attending:5", "attending:5:-1", "attending:5:-1000", "attending:5:9", "attending:5:20"} {
		t.Run(data, func(t *testing.T) {
			pb, usrInfo, cb := forgedSetup(t, data)

			assert.NotPanics(t, func() { pb.ui_Attending(usrInfo, cb) })
		})
	}
}

func Test_goingMoreInfo_forgedData(t *testing.T) {
	for _, data := range []string{"moreinfo", "moreinfo:", "moreinfo:abc"} {
		t.Run(data, func(t *testing.T) {
			pb, usrInfo, cb := forgedSetup(t, data)
			pb.tg.(*TelegramBotMock).On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Maybe()

			assert.NotPanics(t, func() { pb.goingMoreInfo(usrInfo, cb) })
		})
	}
}

func Test_processDateClicks_forgedData(t *testing.T) {
	assert.NotPanics(t, func() { processDateClicks(time.Now(), "calen:month") })
}

func Test_handleUpdate_survivesPanics(t *testing.T) {
	_, _, pb, _ := setupEnvironment(t)

	pb.cmds.AddCB(tgCommands.Callback{DataPrefix: "boom", Public: true, Handler: func(*userManager.UserInfo, *tgbotapi.CallbackQuery) {
		panic("boom")
	}})

	assert.NotPanics(t, func() {
		pb.handleUpdate(tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
			Data: "boom",
			From: &tgbotapi.User{ID: 999},
		}})
	})
}

func Test_handleUpdate_forgedInlineCallback(t *testing.T) {
	_, _, pb, _ := setupEnvironment(t)

	// A button on a message posted inline has no Message, which the private chat handlers rely on.
	called := false
	pb.cmds.AddCB(tgCommands.Callback{DataPrefix: "private", Handler: func(*userManager.UserInfo, *tgbotapi.CallbackQuery) {
		called = true
	}})

	pb.handleUpdate(tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data:            "private",
		From:            &tgbotapi.User{ID: 999},
		InlineMessageID: "abc",
	}})

	assert.False(t, called)
}

func Test_safeGo_survivesPanics(t *testing.T) {
	done := make(chan struct{})
	safeGo(func() {
		defer close(done)
		panic("boom")
	})
	<-done
}
