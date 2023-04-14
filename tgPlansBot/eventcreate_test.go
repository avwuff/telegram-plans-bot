package tgPlansBot

import (
	"furryplansbot.avbrand.com/dbInterface"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func Test_EventCreate(t *testing.T) {

	// Run through all the procedures to create a new event.
	db, tg, _, sU := setupEnvironment(t)

	// expect a start message
	expect(t, tg, "First, send me the name of the event.")
	// send a "/start" command
	sU.sendText("/start")

	// Send the name
	expect(t, tg, "Choose a Date for the event")
	sU.sendText("Test Event")

	expect(t, tg, "Choose a Time")
	sU.sendText("2023-01-01")

	expect(t, tg, "Where does the event take place")
	sU.sendText("20:00")

	// Create a dummy event
	mEvent := simpleEvent(t, "Test Event", "My house")

	// Now the event will be created
	db.On("CreateEvent", int64(1234), "Test Event", mock.Anything, "America/Los_Angeles", "bob", "My house", "en-US").Return(uint(1200), nil)
	db.On("GetEvent", uint(1200), int64(1234)).Return(mEvent, nil)

	expect(t, tg, "Alright")
	sU.sendText("My house")

}

func expect(t *testing.T, tg *TelegramBotMock, text string) {
	tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		m := args.Get(0).(tgbotapi.MessageConfig)
		assert.Contains(t, m.Text, text)
	}).Return(tgbotapi.Message{}, nil).Once()
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
	mEvent.On("DateTime").Return(time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local))
	return mEvent
}
