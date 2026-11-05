package tgPlansBot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fixPicturesSetup works in an empty folder with a picture cache, and doesn't really sleep.
func fixPicturesSetup(t *testing.T, files ...string) *[]time.Duration {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "html", "cache"), 0777))
	for _, file := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "html", "cache", file), []byte("jpg"), 0666))
	}
	t.Chdir(dir)

	sleeps := &[]time.Duration{}
	fixPicturesSleep = func(d time.Duration) { *sleeps = append(*sleeps, d) }
	t.Cleanup(func() { fixPicturesSleep = time.Sleep })
	return sleeps
}

func fixPicturesEvent(t *testing.T, id uint, file string) *dbInterface.DBEventMock {
	ev := dbInterface.NewDBEventMock(t)
	ev.On("ID").Return(id).Maybe()
	ev.On("PictureURL").Return("https://plansbot.avbrand.com/cache/" + file).Maybe()
	return ev
}

// fixPicturesBot makes a bot where none of the events have been given a new picture during the run.
func fixPicturesBot(t *testing.T, tg *TelegramBotMock) *TGPlansBot {
	current := dbInterface.NewDBEventMock(t)
	current.On("PictureFileID").Return("").Maybe()
	db := dbInterface.NewDBFeaturesMock(t)
	db.On("GetEvent", mock.Anything, int64(-1)).Return(current, nil).Maybe()
	return &TGPlansBot{tg: tg, db: db}
}

func sentPhoto(fileID string) tgbotapi.Message {
	return tgbotapi.Message{Photo: []tgbotapi.PhotoSize{
		{FileID: "small", Width: 10, Height: 10},
		{FileID: fileID, Width: 100, Height: 100},
		{FileID: "medium", Width: 50, Height: 50},
	}}
}

func Test_uploadPictures(t *testing.T) {
	t.Run("stores the file ID of the largest size", func(t *testing.T) {
		sleeps := fixPicturesSetup(t, "a.jpg", "b.jpg")

		tg := NewTelegramBotMock(t)
		var sent []tgbotapi.PhotoConfig
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			sent = append(sent, args.Get(0).(tgbotapi.PhotoConfig))
		}).Return(sentPhoto("big"), nil).Twice()

		a := fixPicturesEvent(t, 1, "a.jpg")
		a.On("SetPictureFileID", "big").Return(nil).Once()
		b := fixPicturesEvent(t, 2, "b.jpg")
		b.On("SetPictureFileID", "big").Return(nil).Once()

		pb := fixPicturesBot(t, tg)
		summary := pb.uploadPictures(-100, []dbInterface.DBEvent{a, b})

		assert.Equal(t, "Uploaded 2 pictures.", summary)
		require.Len(t, sent, 2)
		assert.Equal(t, int64(-100), sent[0].ChatID)
		assert.True(t, sent[0].DisableNotification)
		assert.Equal(t, tgbotapi.FilePath(filepath.Join("html", "cache", "a.jpg")), sent[0].File)
		assert.Equal(t, []time.Duration{fixPicturesDelay}, *sleeps, "should wait between uploads, but not after the last")
	})

	t.Run("missing file is reported and the rest continue", func(t *testing.T) {
		fixPicturesSetup(t, "b.jpg")

		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(sentPhoto("big"), nil).Once()

		// The mock fails the test if a file ID is stored for the event with the missing file.
		a := fixPicturesEvent(t, 1, "a.jpg")
		b := fixPicturesEvent(t, 2, "b.jpg")
		b.On("SetPictureFileID", "big").Return(nil).Once()

		pb := fixPicturesBot(t, tg)
		summary := pb.uploadPictures(-100, []dbInterface.DBEvent{a, b})

		assert.Contains(t, summary, "Uploaded 1 pictures.")
		assert.Contains(t, summary, "File missing from disk for 1 events: 1")
	})

	t.Run("server without the files skips them all quickly", func(t *testing.T) {
		sleeps := fixPicturesSetup(t)

		// The mocks fail the test if anything is sent or stored.
		tg := NewTelegramBotMock(t)
		var events []dbInterface.DBEvent
		for i := 0; i < 2000; i++ {
			events = append(events, fixPicturesEvent(t, uint(100000+i), "a.jpg"))
		}

		pb := fixPicturesBot(t, tg)
		summary := pb.uploadPictures(-100, events)

		assert.Empty(t, *sleeps)
		assert.Contains(t, summary, "Uploaded 0 pictures.")
		assert.Contains(t, summary, "File missing from disk for 2000 events: 100000, ")
		assert.Contains(t, summary, "and 1950 more (see the log)")
		assert.NotContains(t, summary, "Stopped after")
		assert.Less(t, len(summary), 4096)
	})

	t.Run("only waits after talking to telegram", func(t *testing.T) {
		sleeps := fixPicturesSetup(t, "b.jpg", "c.jpg")

		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(sentPhoto("big"), nil).Twice()

		b := fixPicturesEvent(t, 2, "b.jpg")
		b.On("SetPictureFileID", "big").Return(nil).Once()
		c := fixPicturesEvent(t, 3, "c.jpg")
		c.On("SetPictureFileID", "big").Return(nil).Once()

		pb := fixPicturesBot(t, tg)
		pb.uploadPictures(-100, []dbInterface.DBEvent{
			fixPicturesEvent(t, 1, "missing.jpg"), b, fixPicturesEvent(t, 4, "missing.jpg"), fixPicturesEvent(t, 5, "missing.jpg"), c,
		})

		// One wait, right after the upload of b, and none once it has moved on to the missing files.
		assert.Equal(t, []time.Duration{fixPicturesDelay}, *sleeps)
	})

	t.Run("rate limited upload is retried after the requested time", func(t *testing.T) {
		sleeps := fixPicturesSetup(t, "a.jpg")

		limited := &tgbotapi.Error{Code: 429, Message: "Too Many Requests", ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 9}}
		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(tgbotapi.Message{}, limited).Once()
		tg.On("Send", mock.Anything).Return(sentPhoto("big"), nil).Once()

		a := fixPicturesEvent(t, 1, "a.jpg")
		a.On("SetPictureFileID", "big").Return(nil).Once()

		pb := fixPicturesBot(t, tg)
		b := fixPicturesEvent(t, 2, "a.jpg")
		b.On("SetPictureFileID", "big").Return(nil).Once()
		tg.On("Send", mock.Anything).Return(sentPhoto("big"), nil).Once()

		assert.Equal(t, "Uploaded 2 pictures.", pb.uploadPictures(-100, []dbInterface.DBEvent{a, b}))

		// It waits as long as it was asked to and a bit more, then slows down for the rest of the run.
		assert.Equal(t, []time.Duration{10 * time.Second, 2 * fixPicturesDelay}, *sleeps)
	})

	t.Run("the delay has a maximum", func(t *testing.T) {
		fixPicturesSetup(t)

		limited := &tgbotapi.Error{Code: 429, ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 1}}
		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(tgbotapi.Message{}, limited).Times(fixPicturesRetries + 1)

		delay := fixPicturesMaxDelay - time.Second
		pb := &TGPlansBot{tg: tg}
		_, err := pb.sendPictureWithRetry(tgbotapi.PhotoConfig{}, &delay)

		assert.ErrorIs(t, err, limited, "should give up after the retries")
		assert.Equal(t, fixPicturesMaxDelay, delay)
	})

	t.Run("a new picture set during the run is not overwritten", func(t *testing.T) {
		sleeps := fixPicturesSetup(t, "a.jpg", "b.jpg")

		current := dbInterface.NewDBEventMock(t)
		current.On("PictureFileID").Return("NEWPICTURE").Once()
		db := dbInterface.NewDBFeaturesMock(t)
		db.On("GetEvent", uint(1), int64(-1)).Return(current, nil).Once()

		notSet := dbInterface.NewDBEventMock(t)
		notSet.On("PictureFileID").Return("").Once()
		db.On("GetEvent", uint(2), int64(-1)).Return(notSet, nil).Once()

		// The mocks fail the test if the picture is uploaded or stored for the first event.
		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(sentPhoto("big"), nil).Once()
		a := fixPicturesEvent(t, 1, "a.jpg")
		b := fixPicturesEvent(t, 2, "b.jpg")
		b.On("SetPictureFileID", "big").Return(nil).Once()

		pb := &TGPlansBot{tg: tg, db: db}
		summary := pb.uploadPictures(-100, []dbInterface.DBEvent{a, b})

		assert.Contains(t, summary, "Uploaded 1 pictures.")
		assert.Contains(t, summary, "Skipped 1 events that got a new picture")
		assert.Empty(t, *sleeps, "skipping shouldn't wait")
	})

	t.Run("stops when everything is failing", func(t *testing.T) {
		fixPicturesSetup(t, "a.jpg")

		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("Bad Request: chat not found")).Times(fixPicturesMaxFailuresInARow)

		var events []dbInterface.DBEvent
		for i := 0; i < fixPicturesMaxFailuresInARow+3; i++ {
			events = append(events, fixPicturesEvent(t, uint(i+1), "a.jpg"))
		}

		pb := fixPicturesBot(t, tg)
		summary := pb.uploadPictures(-100, events)

		assert.Contains(t, summary, "Failed for 5 events: 1, 2, 3, 4, 5")
		assert.Contains(t, summary, "with 3 events not tried")
		assert.Contains(t, summary, "chat not found")
	})

	t.Run("response without a photo is a failure", func(t *testing.T) {
		fixPicturesSetup(t, "a.jpg")

		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

		pb := fixPicturesBot(t, tg)
		assert.Contains(t, pb.uploadPictures(-100, []dbInterface.DBEvent{fixPicturesEvent(t, 1, "a.jpg")}), "Failed for 1 events: 1")
	})
}

func Test_pictureCachePath(t *testing.T) {
	want := filepath.Join("html", "cache", "abc.jpg")

	got, err := pictureCachePath("https://plansbot.avbrand.com/cache/abc.jpg")
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = pictureCachePath("https://evil.example/../../secret/../abc.jpg")
	assert.NoError(t, err)
	assert.Equal(t, want, got, "only the file name should be used")

	_, err = pictureCachePath("https://plansbot.avbrand.com/")
	assert.Error(t, err)
}

func Test_fixPictures_command(t *testing.T) {
	admin := &tgbotapi.Message{Chat: tgbotapi.Chat{ID: limitToUser}}
	usrInfo := &userManager.UserInfo{}

	// quickReply sends with Send, so these check what the user was told.
	reply := func(t *testing.T, tg *TelegramBotMock, want string) {
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			assert.Contains(t, args.Get(0).(tgbotapi.MessageConfig).Text, want)
		}).Return(tgbotapi.Message{}, nil).Once()
	}

	t.Run("only the admin can use it", func(t *testing.T) {
		tg := NewTelegramBotMock(t)
		reply(t, tg, "not available")

		// The database mock fails the test if it is used.
		pb := &TGPlansBot{tg: tg, db: dbInterface.NewDBFeaturesMock(t)}
		pb.fixPictures(usrInfo, &tgbotapi.Message{Chat: tgbotapi.Chat{ID: 12345}}, "-100")
	})

	for _, arg := range []string{"", "abc", "0"} {
		t.Run("needs a channel: '"+arg+"'", func(t *testing.T) {
			tg := NewTelegramBotMock(t)
			reply(t, tg, "ID of the channel")

			pb := &TGPlansBot{tg: tg, db: dbInterface.NewDBFeaturesMock(t)}
			pb.fixPictures(usrInfo, admin, arg)
		})
	}

	t.Run("won't run twice at once", func(t *testing.T) {
		require.True(t, fixPicturesRunning.CompareAndSwap(false, true))
		defer fixPicturesRunning.Store(false)

		tg := NewTelegramBotMock(t)
		reply(t, tg, "Already fixing")

		pb := &TGPlansBot{tg: tg, db: dbInterface.NewDBFeaturesMock(t)}
		pb.fixPictures(usrInfo, admin, "-100")
	})

	t.Run("runs the upload and reports back", func(t *testing.T) {
		fixPicturesSetup(t)

		db := dbInterface.NewDBFeaturesMock(t)
		db.On("EventsMissingPictureFileID").Return([]dbInterface.DBEvent{}, nil).Once()

		done := make(chan string, 2)
		tg := NewTelegramBotMock(t)
		tg.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			done <- args.Get(0).(tgbotapi.MessageConfig).Text
		}).Return(tgbotapi.Message{}, nil).Twice()

		pb := &TGPlansBot{tg: tg, db: db}
		pb.fixPictures(usrInfo, admin, " -100 ")

		assert.True(t, strings.HasPrefix(<-done, "Checking 0 pictures"))
		assert.Equal(t, "Uploaded 0 pictures.", <-done)
		assert.Eventually(t, func() bool { return !fixPicturesRunning.Load() }, time.Second, time.Millisecond)
	})
}

func Test_eventPictureInRichMessage(t *testing.T) {
	pb := &TGPlansBot{}
	loc := eventUILocalizer()

	t.Run("picture goes at the top", func(t *testing.T) {
		rich := pb.makeEventUIRichMessage(eventUIEventFull(t, false, "FILE123"), loc)

		assert.True(t, strings.HasPrefix(rich.HTML, `<img src="tg://photo?id=eventpicture"/><p>`), rich.HTML)
		require.Len(t, rich.Media, 1)
		assert.Equal(t, "eventpicture", rich.Media[0].ID)
	})

	t.Run("no picture has no media", func(t *testing.T) {
		rich := pb.makeEventUIRichMessage(eventUIEvent(t), loc)

		assert.NotContains(t, rich.HTML, "<img")
		assert.Empty(t, rich.Media)
	})
}

func Test_largestPhotoFileID(t *testing.T) {
	assert.Equal(t, "big", largestPhotoFileID(sentPhoto("big").Photo))
	assert.Equal(t, "", largestPhotoFileID(nil))
}
