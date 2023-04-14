package tgCommands

import (
	"furryplansbot.avbrand.com/userManager"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommandList_Process(t *testing.T) {

	tests := []struct {
		name     string
		msg      *tgbotapi.Message
		wantRun  string
		wantText string
		mode     userManager.Mode
	}{
		{
			name:    "initial test",
			msg:     &tgbotapi.Message{Text: "/help"},
			wantRun: "/help",
		},
		{
			name:     "parameters text",
			msg:      &tgbotapi.Message{Text: "/help my hat"},
			wantRun:  "/help",
			wantText: "my hat",
		},
		{
			name:     "underscore test",
			msg:      &tgbotapi.Message{Text: "/edit"},
			wantRun:  "/edit",
			wantText: "",
		},
		{
			name:     "underscore number",
			msg:      &tgbotapi.Message{Text: "/edit_1234"},
			wantRun:  "/edit",
			wantText: "1234",
		},
		{
			name:     "underscore number parameter",
			msg:      &tgbotapi.Message{Text: "/edit_1234 hello"},
			wantRun:  "/edit",
			wantText: "1234 hello",
		},
		{
			name:     "unknown command",
			msg:      &tgbotapi.Message{Text: "/foo"},
			wantRun:  "unknown",
			wantText: "/foo",
		},
		{
			name:     "full text",
			msg:      &tgbotapi.Message{Text: "hi there"},
			wantRun:  "/full",
			wantText: "hi there",
			mode:     userManager.MODE_EDIT_STRING,
		},
		{
			name:     "blank message",
			msg:      &tgbotapi.Message{Text: ""},
			wantRun:  "",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CommandList{}

			// Build the sample commands that all tests use
			runCmd := ""
			runText := ""
			c.Add(Command{
				Command: "/help",
				Handler: func(_ *userManager.UserInfo, msg *tgbotapi.Message, text string) {
					runCmd = "/help"
					runText = text
				},
			})
			c.Add(Command{
				Command:    "/edit",
				Underscore: true,
				Handler: func(_ *userManager.UserInfo, msg *tgbotapi.Message, text string) {
					runCmd = "/edit"
					runText = text
				},
			})
			c.Add(Command{
				Command: "", // full text command
				Mode:    userManager.MODE_EDIT_STRING,
				Handler: func(_ *userManager.UserInfo, msg *tgbotapi.Message, text string) {
					runCmd = "/full"
					runText = text
				},
			})
			c.SetUnknown(func(_ *userManager.UserInfo, msg *tgbotapi.Message, text string) {
				runCmd = "unknown"
				runText = text
			})

			usrInfo := &userManager.UserInfo{
				Eph: &userManager.UserEphemeral{
					UserMode: tt.mode,
				},
			}

			c.Process(usrInfo, tt.msg)

			assert.Equal(t, tt.wantRun, runCmd)
			assert.Equal(t, tt.wantText, runText)
		})
	}
}

func TestCommandList_ProcessCallback(t *testing.T) {

	tests := []struct {
		name     string
		cb       *tgbotapi.CallbackQuery
		wantRun  string
		wantText string
		mode     userManager.Mode
	}{
		{
			name:    "initial test",
			cb:      &tgbotapi.CallbackQuery{Data: "basic:123"},
			wantRun: "basic",
		},
		{
			name:    "no colon",
			cb:      &tgbotapi.CallbackQuery{Data: "basic"},
			wantRun: "basic",
		},
		{
			name:    "unknown",
			cb:      &tgbotapi.CallbackQuery{Data: "unknown"},
			wantRun: "",
		},
		{
			name:    "mode",
			cb:      &tgbotapi.CallbackQuery{Data: "date"},
			wantRun: "date",
			mode:    userManager.MODE_EDIT_STRING,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CommandList{}

			// Build the sample commands that all tests use
			runCmd := ""
			c.AddCB(Callback{
				DataPrefix: "basic",
				Handler: func(_ *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
					runCmd = "basic"
				},
			})
			c.AddCB(Callback{
				DataPrefix: "date",
				Mode:       userManager.MODE_EDIT_STRING,
				Handler: func(_ *userManager.UserInfo, cb *tgbotapi.CallbackQuery) {
					runCmd = "date"
				},
			})

			usrInfo := &userManager.UserInfo{
				Eph: &userManager.UserEphemeral{
					UserMode: tt.mode,
				},
			}

			c.ProcessCallback(usrInfo, tt.cb)

			assert.Equal(t, tt.wantRun, runCmd)
		})
	}
}
