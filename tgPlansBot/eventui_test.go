package tgPlansBot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"furryplansbot.avbrand.com/dbInterface"
	"furryplansbot.avbrand.com/localizer"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// eventUIEvent makes a simple event that can be displayed in the event UI.
func eventUIEvent(t *testing.T, attending ...*dbInterface.Attend) *dbInterface.DBEventMock {
	return eventUIEventNames(t, false, attending...)
}

// eventUIEventNames is the same as eventUIEvent, and allows the attendee names to be hidden.
func eventUIEventNames(t *testing.T, hideNames bool, attending ...*dbInterface.Attend) *dbInterface.DBEventMock {
	return eventUIEventFull(t, hideNames, "", attending...)
}

// eventUIEventFull is the same as eventUIEvent, and allows all the options to be set.
func eventUIEventFull(t *testing.T, hideNames bool, pictureFileID string, attending ...*dbInterface.Attend) *dbInterface.DBEventMock {
	ev := dbInterface.NewDBEventMock(t)
	ev.On("PictureFileID").Return(pictureFileID).Maybe()
	ev.On("Name").Return("Test Event").Maybe()
	ev.On("OwnerName").Return("bob").Maybe()
	ev.On("Location").Return("My house").Maybe()
	ev.On("Notes").Return("").Maybe()
	ev.On("ID").Return(uint(1200)).Maybe()
	ev.On("DateTime").Return(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).Maybe()
	ev.On("EndDateTime").Return(time.Time{}).Maybe()
	ev.On("MaxAttendees").Return(0).Maybe()
	ev.On("MaxGuests").Return(0).Maybe()
	ev.On("TotalCost").Return(0).Maybe()
	ev.On("GetAttending", int64(-1)).Return(attending, nil).Maybe()
	ev.On("Suitwalk").Return(false).Maybe()
	ev.On("HideNames").Return(hideNames).Maybe()
	ev.On("Closed").Return(false).Maybe()
	ev.On("DisableMaybe").Return(false).Maybe()
	ev.On("SharingAllowed").Return(false).Maybe()
	return ev
}

// captureEdits answers each Request with the next error, and records the edits it was given.
func captureEdits(t *testing.T, errs ...error) (*TGPlansBot, *[]tgbotapi.EditMessageTextConfig) {
	tg := NewTelegramBotMock(t)
	edits := &[]tgbotapi.EditMessageTextConfig{}
	for _, err := range errs {
		tg.On("Request", mock.Anything).Run(func(args mock.Arguments) {
			*edits = append(*edits, args.Get(0).(tgbotapi.EditMessageTextConfig))
		}).Return(&tgbotapi.APIResponse{}, err).Once()
	}
	return &TGPlansBot{tg: tg}, edits
}

// eventUIPeople makes a number of people with the same attending status.
func eventUIPeople(count int, canAttend dbInterface.CanAttend, firstId int64) []*dbInterface.Attend {
	people := make([]*dbInterface.Attend, count)
	for i := range people {
		id := firstId + int64(i)
		people[i] = &dbInterface.Attend{UserID: id, UserName: fmt.Sprintf("P%d", id), CanAttend: int(canAttend)}
	}
	return people
}

func Test_collapsedLists(t *testing.T) {
	localizer.InitLang()
	loc := localizer.FromLanguage("en-US")
	pb := &TGPlansBot{}

	tests := []struct {
		name                          string
		going, maybe, not             int
		wantGoing, wantMaybe, wantNot bool
	}{
		{name: "all small", going: 2, maybe: 1, not: 1},
		{name: "exactly the limit stays open", going: 50, maybe: 50, not: 50},
		{name: "big attending collapses everything", going: 51, maybe: 1, not: 1, wantGoing: true, wantMaybe: true, wantNot: true},
		{name: "big maybe collapses only itself", going: 2, maybe: 51, not: 1, wantMaybe: true},
		{name: "big not going collapses only itself", going: 2, maybe: 1, not: 51, wantNot: true},
		{name: "big maybe with nobody attending", going: 0, maybe: 51, not: 1, wantMaybe: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var people []*dbInterface.Attend
			people = append(people, eventUIPeople(tt.going, dbInterface.CANATTEND_YES, 1000)...)
			people = append(people, eventUIPeople(tt.maybe, dbInterface.CANATTEND_MAYBE, 2000)...)
			people = append(people, eventUIPeople(tt.not, dbInterface.CANATTEND_NO, 3000)...)

			rich := pb.makeEventUIRichHtml(eventUIEvent(t, people...), loc)
			assert.NotContains(t, rich, "\n")
			assert.NotContains(t, rich, "<details open")

			check := func(heading string, count int, want bool) {
				if count == 0 {
					assert.NotContains(t, rich, heading)
					return
				}
				collapsed := fmt.Sprintf("<details><summary><b>%s: %d</b></summary>", heading, count)
				if want {
					assert.Contains(t, rich, collapsed)
				} else {
					assert.NotContains(t, rich, collapsed)
					assert.Contains(t, rich, fmt.Sprintf("<b>%s: %d</b><br>", heading, count))
				}
			}
			check("Attending", tt.going, tt.wantGoing)
			check("Maybe", tt.maybe, tt.wantMaybe)
			check("Can't make it", tt.not, tt.wantNot)

			// Classic messages don't support details, no matter how long the lists are.
			assert.NotContains(t, pb.makeEventUIText(eventUIEvent(t, people...), loc), "details")
		})
	}

	t.Run("plus ones count towards attending", func(t *testing.T) {
		people := []*dbInterface.Attend{
			{UserID: 1, UserName: "Alice", CanAttend: int(dbInterface.CANATTEND_YES), PlusMany: 50, Guests: []string{"Gus"}},
		}
		rich := pb.makeEventUIRichHtml(eventUIEvent(t, people...), loc)

		assert.Contains(t, rich, `<details><summary><b>Attending: 51</b></summary><p>&nbsp;- <a href="tg://user?id=1">Alice</a> <b>+50</b><br>&nbsp;&nbsp;&nbsp;- <i>Gus</i></p></details>`)
	})

	t.Run("hidden names have nothing to expand", func(t *testing.T) {
		people := eventUIPeople(51, dbInterface.CANATTEND_YES, 1000)
		rich := pb.makeEventUIRichHtml(eventUIEventNames(t, true, people...), loc)

		assert.NotContains(t, rich, "details")
		assert.NotContains(t, rich, "P1000")
		assert.Contains(t, rich, "<b>Attending: 51</b>")
	})
}

func Test_makeEventUI_inline(t *testing.T) {
	localizer.InitLang()
	loc := localizer.FromLanguage("en-US")

	t.Run("sent as rich message", func(t *testing.T) {
		pb, edits := captureEdits(t, nil)
		_, err := pb.makeEventUI(0, eventUIEvent(t), loc, "inline1")

		require.NoError(t, err)
		require.Len(t, *edits, 1)
		edit := (*edits)[0]
		assert.Equal(t, "inline1", edit.InlineMessageID)
		assert.Empty(t, edit.Text)
		assert.Empty(t, edit.ParseMode)
		assert.Contains(t, edit.RichMessage.HTML, "<b>Test Event</b>")
		assert.NotContains(t, edit.RichMessage.HTML, "\n")
		assert.NotNil(t, edit.ReplyMarkup)
	})

	t.Run("rejected rich message falls back to classic", func(t *testing.T) {
		pb, edits := captureEdits(t, errors.New("Bad Request: can't parse rich message"), nil)
		_, err := pb.makeEventUI(0, eventUIEvent(t), loc, "inline1")

		require.NoError(t, err)
		require.Len(t, *edits, 2)
		assert.NotEmpty(t, (*edits)[0].RichMessage.HTML)
		assert.Empty(t, (*edits)[1].RichMessage.HTML)
		assert.Contains(t, (*edits)[1].Text, "<b>Test Event</b>")
		assert.True(t, (*edits)[1].LinkPreviewOptions.IsDisabled)
	})

	for _, msg := range []string{"Too Many Requests: retry after 5", "MESSAGE_ID_INVALID", "Bad Request: message is not modified"} {
		t.Run("no fallback on "+msg, func(t *testing.T) {
			pb, edits := captureEdits(t, errors.New(msg))
			_, err := pb.makeEventUI(0, eventUIEvent(t), loc, "inline1")

			assert.ErrorContains(t, err, msg)
			assert.Len(t, *edits, 1)
		})
	}
}

func Test_makeEventUIRegular(t *testing.T) {
	localizer.InitLang()
	loc := localizer.FromLanguage("en-US")

	t.Run("sent as rich message", func(t *testing.T) {
		pb, edits := captureEdits(t, nil)
		_, err := pb.makeEventUIRegular(55, eventUIEvent(t), loc, 66)

		require.NoError(t, err)
		require.Len(t, *edits, 1)
		assert.Equal(t, int64(55), (*edits)[0].ChatID)
		assert.Equal(t, 66, (*edits)[0].MessageID)
		assert.NotEmpty(t, (*edits)[0].RichMessage.HTML)
	})

	t.Run("no fallback when message is gone", func(t *testing.T) {
		pb, edits := captureEdits(t, errors.New("Bad Request: message to edit not found"))
		_, err := pb.makeEventUIRegular(55, eventUIEvent(t), loc, 66)

		assert.Error(t, err)
		assert.Len(t, *edits, 1)
	})
}

func Test_requestEventEdit(t *testing.T) {
	t.Run("nil response does not crash", func(t *testing.T) {
		tg := NewTelegramBotMock(t)
		tg.On("Request", mock.Anything).Return(nil, errors.New("network down")).Once()
		pb := &TGPlansBot{tg: tg}

		retry, err := pb.requestEventEdit(tgbotapi.NewEditMessageText(1, 2, "x"))
		assert.Error(t, err)
		assert.Equal(t, 0, retry)
	})

	t.Run("retry after is returned", func(t *testing.T) {
		tg := NewTelegramBotMock(t)
		tg.On("Request", mock.Anything).Return(&tgbotapi.APIResponse{
			Parameters: &tgbotapi.ResponseParameters{RetryAfter: 7},
		}, errors.New("Too Many Requests")).Once()
		pb := &TGPlansBot{tg: tg}

		retry, err := pb.requestEventEdit(tgbotapi.NewEditMessageText(1, 2, "x"))
		assert.Error(t, err)
		assert.Equal(t, 7, retry)
	})
}

// wireBot sends requests through a real bot instead of the mock.
type wireBot struct {
	*TelegramBotMock
	bot *tgbotapi.BotAPI
}

func (w wireBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return w.bot.Request(c)
}

// Test_richEditOnTheWire checks what the real library sends to Telegram for a rich edit of an inline message.
func Test_richEditOnTheWire(t *testing.T) {
	localizer.InitLang()
	loc := localizer.FromLanguage("en-US")

	var form url.Values
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"t","username":"t"}}`))
			return
		}
		path = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			_ = r.ParseForm()
		}
		form = r.Form
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("token", srv.URL+"/bot%s/%s")
	require.NoError(t, err)

	pb := &TGPlansBot{tg: wireBot{TelegramBotMock: NewTelegramBotMock(t), bot: bot}}
	_, err = pb.makeEventUI(0, eventUIEvent(t), loc, "inline1")
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(path, "/editMessageText"), path)
	assert.Equal(t, "inline1", form.Get("inline_message_id"))
	assert.Empty(t, form.Get("text"))
	assert.Empty(t, form.Get("parse_mode"))
	assert.NotEmpty(t, form.Get("reply_markup"))

	var rich map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(form.Get("rich_message")), &rich))
	assert.Contains(t, rich["html"], "<b>Test Event</b>")
}

// eventUILocalizer gets the localizer used by the event UI tests.
func eventUILocalizer() *localizer.Localizer {
	localizer.InitLang()
	return localizer.FromLanguage("en-US")
}

// Test_richPictureOnTheWire checks what the real library sends to Telegram for a picture, for both kinds of edit.
func Test_richPictureOnTheWire(t *testing.T) {
	loc := eventUILocalizer()

	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"t","username":"t"}}`))
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			_ = r.ParseForm()
		}
		form = r.Form
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint("token", srv.URL+"/bot%s/%s")
	require.NoError(t, err)
	pb := &TGPlansBot{tg: wireBot{TelegramBotMock: NewTelegramBotMock(t), bot: bot}}

	check := func(t *testing.T) {
		var rich struct {
			HTML  string `json:"html"`
			Media []struct {
				ID    string `json:"id"`
				Media struct {
					Type  string `json:"type"`
					Media string `json:"media"`
				} `json:"media"`
			} `json:"media"`
		}
		require.NoError(t, json.Unmarshal([]byte(form.Get("rich_message")), &rich), form.Get("rich_message"))
		assert.Contains(t, rich.HTML, `<img src="tg://photo?id=eventpicture"/>`)
		require.Len(t, rich.Media, 1)
		assert.Equal(t, "eventpicture", rich.Media[0].ID)
		assert.Equal(t, "photo", rich.Media[0].Media.Type)
		assert.Equal(t, "FILE123", rich.Media[0].Media.Media)
	}

	t.Run("inline", func(t *testing.T) {
		_, err := pb.makeEventUI(0, eventUIEventFull(t, false, "FILE123"), loc, "inline1")
		require.NoError(t, err)
		check(t)
	})

	t.Run("regular", func(t *testing.T) {
		_, err := pb.makeEventUIRegular(55, eventUIEventFull(t, false, "FILE123"), loc, 66)
		require.NoError(t, err)
		check(t)
	})
}
