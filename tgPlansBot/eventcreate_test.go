package tgPlansBot

import (
	"encoding/json"
	"furryplansbot.avbrand.com/dbInterface"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func Test_EventCreate(t *testing.T) {

	type tCmd struct {
		send       string // as if they had typed this text
		sendbutton string // as if they had clicked this button
		expect     string
		exrequest  string
	}

	dummy1 := func(t *testing.T, db *dbInterface.DBFeaturesMock) {
		// Create a dummy event
		mEvent := simpleEvent(t, "Test Event", "My house")

		loc, _ := time.LoadLocation("America/Los_Angeles")

		// Now the event will be created
		db.On("CreateEvent", int64(1234), "Test Event", time.Date(2023, 1, 1, 20, 0, 0, 0, loc), "America/Los_Angeles", "bob", "My house", "en-US", "Some notes").Return(uint(1200), nil)
		db.On("GetEvent", uint(1200), int64(1234)).Return(mEvent, nil)
	}
	dummy2 := func(t *testing.T, db *dbInterface.DBFeaturesMock) {
		// Create a dummy event
		mEvent := simpleEvent(t, "Test Event", "My house")

		loc, _ := time.LoadLocation("America/Los_Angeles")

		// Now the event will be created
		db.On("CreateEvent", int64(1234), "Test Event", time.Date(2023, 1, 1, 20, 0, 0, 0, loc), "America/Los_Angeles", "bob", "My house", "en-US", "").Return(uint(1200), nil)
		db.On("GetEvent", uint(1200), int64(1234)).Return(mEvent, nil)
	}

	tests := []struct {
		name string
		cmds []tCmd
		on   func(t *testing.T, db *dbInterface.DBFeaturesMock)
	}{
		{
			name: "first event create",
			cmds: []tCmd{
				{send: "/start", expect: "First, send me the name of the event."},
				{send: "Test Event", expect: "Choose a Date for the event"},
				{send: "2023-01-01", expect: "Choose a Time"},
				{send: "20:00", expect: "Where does the event take place"},
				{send: "My house", expect: "Specify any additional notes"},
				{send: "Some notes", expect: "Alright"},
			},
			on: dummy1,
		},
		{
			name: "Skip notes",
			cmds: []tCmd{
				{send: "/start", expect: "First, send me the name of the event."},
				{send: "Test Event", expect: "Choose a Date for the event"},
				{send: "2023-01-01", expect: "Choose a Time"},
				{send: "20:00", expect: "Where does the event take place"},
				{send: "My house", expect: "Specify any additional notes"},
				{send: "/skip", expect: "Alright"},
			},
			on: dummy2,
		},
		{
			name: "choosing date via calendar",
			cmds: []tCmd{
				{send: "/start", expect: "First, send me the name of the event."},
				{send: "Test Event", expect: "Choose a Date for the event"},
				{sendbutton: "calen:sel:2023-01-01", exrequest: "Choose a Date for the event"},
				{sendbutton: "calen:finish", exrequest: "Date selected", expect: "Choose a Time"},
				{sendbutton: "time:hour:20", exrequest: "Choose a Time"},
				{sendbutton: "time:minute:0", exrequest: "Choose a Time"},
				{sendbutton: "time:finish", exrequest: "Time selected", expect: "Where does the event take place"},
				{send: "My house", expect: "Specify any additional notes"},
				{send: "Some notes", expect: "Alright"},
			},
			on: dummy1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run through all the procedures to create a new event.
			db, tg, _, sU := setupEnvironment(t)

			tt.on(t, db)
			for _, cmd := range tt.cmds {
				if cmd.expect != "" {
					expect(t, tg, cmd.expect)
				}
				if cmd.exrequest != "" {
					expectReq(t, tg, cmd.exrequest)
				}
				if cmd.send != "" {
					sU.sendText(cmd.send)
				}
				if cmd.sendbutton != "" {
					sU.sendButton(cmd.sendbutton)
				}

			}
		})
	}

}

func expect(t *testing.T, tg *TelegramBotMock, text string) {
	tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		m := args.Get(0).(tgbotapi.MessageConfig)
		assert.Contains(t, m.Text, text)
	}).Return(tgbotapi.Message{}, nil).Once()
}
func expectReq(t *testing.T, tg *TelegramBotMock, text string) {
	tg.On("Request", mock.Anything).Run(func(args mock.Arguments) {
		m := args.Get(0)
		s, _ := json.Marshal(m)
		var dd map[string]interface{}
		_ = json.Unmarshal(s, &dd)

		assert.Contains(t, dd["Text"], text)
	}).Return(&tgbotapi.APIResponse{Ok: true}, nil).Once()
}

func simpleEvent(t *testing.T, name string, location string) *dbInterface.DBEventMock {
	mEvent := dbInterface.NewDBEventMock(t)
	mEvent.On("Name").Return(name)
	mEvent.On("Location").Return(location)
	mEvent.On("Language").Return("en-US")
	mEvent.On("OwnerName").Return("bob")
	mEvent.On("Notes").Return("Notes here")
	mEvent.On("ID").Return(uint(1200))
	mEvent.On("Suitwalk").Return(false)
	mEvent.On("DisableMaybe").Return(false)
	mEvent.On("SharingAllowed").Return(false)
	mEvent.On("MaxAttendees").Return(0)
	mEvent.On("Closed").Return(false)
	mEvent.On("PictureURL").Return("")
	mEvent.On("DateTime").Return(time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local))
	mEvent.On("EndDateTime").Return(time.Time{})
	return mEvent
}
