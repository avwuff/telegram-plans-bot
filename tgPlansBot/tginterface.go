package tgPlansBot

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

//go:generate go run github.com/vektra/mockery/v2 --name=TelegramBot --structname TelegramBotMock --filename telegrambot_mock.go --inpackage

type TelegramBot interface {
	Init() error
	Listen(ctx context.Context, handler func(update tgbotapi.Update))
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	AnswerInlineQuery(c tgbotapi.InlineConfig) (*tgbotapi.APIResponse, error)
	AnswerCallbackQuery(c tgbotapi.CallbackConfig) (*tgbotapi.APIResponse, error)
	SetMyCommands(cmds tgbotapi.SetMyCommandsConfig) (*tgbotapi.APIResponse, error)
	GetFile(config tgbotapi.FileConfig) (tgbotapi.File, error)
	GetFileDirectURL(fileID string) (string, error)
}

const (
	ParseModeHtml = "HTML"
)
