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
	return eventUIEventOptions(t, eventUIOptions{hideNames: hideNames, pictureFileID: pictureFileID}, attending...)
}

// eventUIOptions are the things about an event that a test can change.
type eventUIOptions struct {
	hideNames     bool
	pictureFileID string
	notes         string
	maxAttendees  int
	maxGuests     int
	totalCost     int
	donations     float64
	closed        bool
	suitwalk      bool
	attendErr     error
}

// eventUIEventOptions is the same as eventUIEvent, and allows the options to be set.
func eventUIEventOptions(t *testing.T, opt eventUIOptions, attending ...*dbInterface.Attend) *dbInterface.DBEventMock {
	ev := dbInterface.NewDBEventMock(t)
	ev.On("PictureFileID").Return(opt.pictureFileID).Maybe()
	ev.On("Name").Return("Test Event").Maybe()
	ev.On("OwnerName").Return("bob").Maybe()
	ev.On("Location").Return("My house").Maybe()
	ev.On("Notes").Return(opt.notes).Maybe()
	ev.On("ID").Return(uint(1200)).Maybe()
	ev.On("DateTime").Return(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).Maybe()
	ev.On("EndDateTime").Return(time.Time{}).Maybe()
	ev.On("MaxAttendees").Return(opt.maxAttendees).Maybe()
	ev.On("MaxGuests").Return(opt.maxGuests).Maybe()
	ev.On("TotalCost").Return(opt.totalCost).Maybe()
	ev.On("GetDonationTotal").Return(opt.donations, nil).Maybe()
	ev.On("GetAttending", int64(-1)).Return(attending, opt.attendErr).Maybe()
	ev.On("Suitwalk").Return(opt.suitwalk).Maybe()
	ev.On("HideNames").Return(opt.hideNames).Maybe()
	ev.On("Closed").Return(opt.closed).Maybe()
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

			check := func(heading string, count int, want bool, open string) {
				collapsed := fmt.Sprintf("<details><summary><b>%s: %d</b></summary>", heading, count)
				if want {
					assert.Contains(t, rich, collapsed)
				} else {
					assert.NotContains(t, rich, collapsed)
					assert.Contains(t, rich, fmt.Sprintf(open, heading, count))
				}
			}
			if tt.going == 0 {
				assert.Contains(t, rich, "<p><b>Attending: 0 · Be the first!</b></p>")
			} else {
				check("Attending", tt.going, tt.wantGoing, "<p><b>%s: %d</b></p><ul>")
			}
			check("Maybe", tt.maybe, tt.wantMaybe, "<p><b>%s: %d</b> — ")
			check("Can't make it", tt.not, tt.wantNot, "<p><b>%s: %d</b> — ")

			// Classic messages don't support details, no matter how long the lists are.
			assert.NotContains(t, pb.makeEventUIText(eventUIEvent(t, people...), loc), "details")
		})
	}

	t.Run("plus ones count towards attending", func(t *testing.T) {
		people := []*dbInterface.Attend{
			{UserID: 1, UserName: "Alice", CanAttend: int(dbInterface.CANATTEND_YES), PlusMany: 50, Guests: []string{"Gus"}},
		}
		rich := pb.makeEventUIRichHtml(eventUIEvent(t, people...), loc)

		assert.Contains(t, rich, `<details><summary><b>Attending: 51</b></summary><ul><li><a href="tg://user?id=1">Alice</a> <b>+50</b><ul><li><i>Gus</i></li></ul></li></ul></details>`)
	})

	t.Run("hidden names have nothing to expand", func(t *testing.T) {
		people := eventUIPeople(51, dbInterface.CANATTEND_YES, 1000)
		rich := pb.makeEventUIRichHtml(eventUIEventNames(t, true, people...), loc)

		assert.NotContains(t, rich, "details")
		assert.NotContains(t, rich, "P1000")
		assert.Contains(t, rich, "<b>Attending: 51</b>")
	})
}

func Test_richEventLayout(t *testing.T) {
	loc := eventUILocalizer()
	now := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	going := eventUIPeople(12, dbInterface.CANATTEND_YES, 1000)

	t.Run("simple event", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{notes: "Line one\nLine two"}), loc, now)

		assert.True(t, strings.HasPrefix(rich, "<h1>Test Event</h1><h4>Hosted by bob</h4><p>📅 Sun, Jan 1, 12:00 AM · "), rich)
		assert.Contains(t, rich, fmt.Sprintf(`<tg-time unix="%d" format="r">Sun, Jan 1, 12:00 AM</tg-time>`, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()))
		assert.Contains(t, rich, `<br>📍 <a href="https://www.google.com/maps/search/?api=1&query=My+house">My house</a></p>`)
		assert.Contains(t, rich, "<blockquote>Line one<br>Line two</blockquote>")
		assert.NotContains(t, rich, "Notes:")
		assert.NotContains(t, rich, "Max Attendees")
		assert.NotContains(t, rich, "<footer>")
		assert.NotContains(t, rich, "Can you go")
	})

	t.Run("attending comes first even when empty", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEvent(t, eventUIPeople(1, dbInterface.CANATTEND_NO, 3000)...), loc, now)

		assert.Contains(t, rich, `<p><b>Attending: 0 · Be the first!</b></p><p><b>Can't make it: 1</b> — <a href="tg://user?id=3000">P3000</a></p>`)
	})

	t.Run("spots left", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{maxAttendees: 20}, going...), loc, now)

		assert.Contains(t, rich, "👥 12 of 20 spots taken · 8 left</p>")
		assert.NotContains(t, rich, "<footer>")
	})

	t.Run("full", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{maxAttendees: 12}, going...), loc, now)

		assert.Contains(t, rich, "👥 12 of 12 spots taken · Full</p>")
		assert.True(t, strings.HasSuffix(rich, "<footer>This event is full.</footer>"), rich)
	})

	t.Run("plus ones and fursuit walk roles take up spots", func(t *testing.T) {
		people := []*dbInterface.Attend{
			{UserID: 1, UserName: "A", CanAttend: int(dbInterface.CANATTEND_SUITING), PlusMany: 1},
			{UserID: 2, UserName: "B", CanAttend: int(dbInterface.CANATTEND_PHOTOGRAPHER)},
			{UserID: 3, UserName: "C", CanAttend: int(dbInterface.CANATTEND_YES)},
			{UserID: 4, UserName: "D", CanAttend: int(dbInterface.CANATTEND_MAYBE)},
		}
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{maxAttendees: 10, suitwalk: true}, people...), loc, now)

		assert.Contains(t, rich, "👥 4 of 10 spots taken · 6 left</p>")
	})

	t.Run("closed", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{closed: true, maxAttendees: 1}), loc, now)

		assert.True(t, strings.HasSuffix(rich, "<footer>This event is closed.</footer>"), rich)
		assert.Contains(t, rich, "<p><b>Attending: 0</b></p>")
		assert.NotContains(t, rich, "Be the first")
	})

	t.Run("attendance error", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{maxAttendees: 5, attendErr: errors.New("db")}), loc, now)

		assert.Contains(t, rich, "<p>Unable to get list of attending people.</p>")
		assert.NotContains(t, rich, "spots taken")
		assert.NotContains(t, rich, "Attending:")
		assert.NotContains(t, rich, "<footer>")
	})

	t.Run("fursuit walk lists collapse too", func(t *testing.T) {
		people := eventUIPeople(51, dbInterface.CANATTEND_SUITING, 1000)
		people = append(people, eventUIPeople(1, dbInterface.CANATTEND_PHOTOGRAPHER, 2000)...)
		people = append(people, eventUIPeople(1, dbInterface.CANATTEND_MAYBE, 3000)...)
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{suitwalk: true}, people...), loc, now)

		assert.Contains(t, rich, "<details><summary><b>Suiting: 51</b></summary><ul>")
		assert.Contains(t, rich, "<p><b>Photographers: 1</b></p><ul>")
		assert.Contains(t, rich, "<details><summary><b>Maybe: 1</b></summary><p>")
	})

	t.Run("empty fursuit walk", func(t *testing.T) {
		rich := buildEventUIRichHtml(eventUIEventOptions(t, eventUIOptions{suitwalk: true}), loc, now)

		assert.Contains(t, rich, "<p><b>Suiting: 0 · Be the first!</b></p>")
		assert.NotContains(t, rich, "Attending")
	})

	t.Run("huge lists use plain lines to stay within the block limit", func(t *testing.T) {
		at := eventUIEvent(t, eventUIPeople(richListRowsLimit, dbInterface.CANATTEND_YES, 1000)...)
		assert.Contains(t, buildEventUIRichHtml(at, loc, now), "<ul>")

		over := eventUIEvent(t, eventUIPeople(richListRowsLimit+1, dbInterface.CANATTEND_YES, 1000)...)
		rich := buildEventUIRichHtml(over, loc, now)
		assert.NotContains(t, rich, "<ul>")
		assert.Contains(t, rich, `</summary><p>&nbsp;- <a href="tg://user?id=1000">P1000</a><br>`)
	})
}

func Test_classicEventText(t *testing.T) {
	loc := eventUILocalizer()
	pb := &TGPlansBot{}

	t.Run("open event has no status", func(t *testing.T) {
		text := pb.makeEventUIText(eventUIEvent(t, eventUIPeople(1, dbInterface.CANATTEND_YES, 1000)...), loc)

		assert.Contains(t, text, "<b>Test Event</b> hosted by bob\n")
		assert.Contains(t, text, "\n<b>Attending: 1</b>\n - <a href=\"tg://user?id=1000\">P1000</a>")
		assert.NotContains(t, text, "Can you go")
		assert.False(t, strings.HasSuffix(text, "\n"), text)
	})

	t.Run("closed", func(t *testing.T) {
		text := pb.makeEventUIText(eventUIEventOptions(t, eventUIOptions{closed: true}), loc)
		assert.True(t, strings.HasSuffix(text, "<i>This event is closed.</i>"), text)
	})

	t.Run("full", func(t *testing.T) {
		text := pb.makeEventUIText(eventUIEventOptions(t, eventUIOptions{maxAttendees: 1}, eventUIPeople(1, dbInterface.CANATTEND_YES, 1000)...), loc)
		assert.True(t, strings.HasSuffix(text, "<i>This event is full.</i>"), text)
	})

	t.Run("attendance error", func(t *testing.T) {
		text := pb.makeEventUIText(eventUIEventOptions(t, eventUIOptions{attendErr: errors.New("db")}), loc)
		assert.Contains(t, text, "Unable to get list of attending people.")
	})
}

func Test_eventFunding(t *testing.T) {
	loc := eventUILocalizer()
	pb := &TGPlansBot{saltValue: "salt"}

	hasDonate := func(buttons tgbotapi.InlineKeyboardMarkup) bool {
		for _, row := range buttons.InlineKeyboard {
			for _, button := range row {
				if strings.Contains(button.Text, "Donate") {
					return true
				}
			}
		}
		return false
	}

	t.Run("still collecting", func(t *testing.T) {
		ev := eventUIEventOptions(t, eventUIOptions{totalCost: 100, donations: 99.5})
		assert.Contains(t, pb.makeEventUIRichHtml(ev, loc), "We're at <b>99.50</b> of <b>100</b>")
		assert.True(t, hasDonate(pb.eventUIButtons(ev, loc)))
	})

	t.Run("funded", func(t *testing.T) {
		ev := eventUIEventOptions(t, eventUIOptions{totalCost: 100, donations: 100})
		rich := pb.makeEventUIRichHtml(ev, loc)
		assert.Contains(t, rich, "Fully funded, thank you!")
		assert.NotContains(t, rich, "Help cover")
		assert.Contains(t, pb.makeEventUIText(ev, loc), "Fully funded, thank you!")
		assert.False(t, hasDonate(pb.eventUIButtons(ev, loc)))
	})
}

func Test_guestButtonLabels(t *testing.T) {
	loc := eventUILocalizer()
	pb := &TGPlansBot{saltValue: "salt"}

	buttons := pb.eventUIButtons(eventUIEventOptions(t, eventUIOptions{maxGuests: 3}), loc)
	var labels []string
	for _, button := range buttons.InlineKeyboard[1] {
		labels = append(labels, button.Text)
	}
	require.Len(t, labels, 3)
	assert.True(t, strings.HasSuffix(labels[0], " +1"), labels[0])
	assert.True(t, strings.HasSuffix(labels[1], " +2"), labels[1])
	assert.True(t, strings.HasSuffix(labels[2], " +3"), labels[2])
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
		assert.Contains(t, edit.RichMessage.HTML, "<h1>Test Event</h1>")
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
	assert.Contains(t, rich["html"], "<h1>Test Event</h1>")
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

func Test_eventPictureInRichMessage(t *testing.T) {
	pb := &TGPlansBot{}
	loc := eventUILocalizer()

	t.Run("picture goes at the top", func(t *testing.T) {
		rich := pb.makeEventUIRichMessage(eventUIEventFull(t, false, "FILE123"), loc)

		assert.True(t, strings.HasPrefix(rich.HTML, `<img src="tg://photo?id=eventpicture"/><h1>`), rich.HTML)
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
	photos := []tgbotapi.PhotoSize{
		{FileID: "small", Width: 10, Height: 10},
		{FileID: "big", Width: 100, Height: 100},
		{FileID: "medium", Width: 50, Height: 50},
	}
	assert.Equal(t, "big", largestPhotoFileID(photos))
	assert.Equal(t, "", largestPhotoFileID(nil))
}
