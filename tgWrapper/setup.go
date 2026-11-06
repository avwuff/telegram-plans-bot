package tgWrapper

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Telegram struct {
	bot *tgbotapi.BotAPI
	key string
}

// tokenSafeClient removes the URL from network errors. The bot token is part of every Telegram URL,
// so these errors would otherwise put it in the logs.
type tokenSafeClient struct {
	client tgbotapi.HTTPClient
}

func (c tokenSafeClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return resp, fmt.Errorf("telegram request: %w", urlErr.Err)
	}
	return resp, err
}

func New() *Telegram {
	return &Telegram{}
}

func (t *Telegram) LoadKeyFromFile(file string) error {
	dat, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	t.key = strings.TrimSpace(string(dat))
	return nil
}

func (t *Telegram) Init() error {
	var err error
	tgbotapi.SetLogger(log.Default())
	t.bot, err = tgbotapi.NewBotAPIWithOptions(t.key, tgbotapi.WithHTTPClient(tokenSafeClient{client: &http.Client{}}))

	if err != nil {
		return err
	}
	return nil
}

// Listen is designed to be called as a Gofunc.
func (t *Telegram) Listen(ctx context.Context, handler func(update tgbotapi.Update)) {
	if t.bot == nil {
		panic("Listen called without Init")
	}

	var updates tgbotapi.UpdatesChannel

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates = t.bot.GetUpdatesChan(u)

	for {
		// get either the done context or the next update
		select {
		case <-ctx.Done():
			// exit the function
			t.bot.StopReceivingUpdates()
			return

		case update := <-updates:
			// send it to the handler
			// TODO: Do we want to do a multithreaded pool of handlers for these?  So one update doesn't block. Maybe later.
			handler(update)
		}
	}
}

// Send just wraps the send function of the TGBotApi
func (t *Telegram) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return t.bot.Send(c)
}

func (t *Telegram) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return t.bot.Request(c)
}

func (t *Telegram) AnswerInlineQuery(c tgbotapi.InlineConfig) (*tgbotapi.APIResponse, error) {
	return t.bot.Request(c)
}
func (t *Telegram) AnswerCallbackQuery(c tgbotapi.CallbackConfig) (*tgbotapi.APIResponse, error) {
	return t.bot.Request(c)
}

func (t *Telegram) SetMyCommands(cmds tgbotapi.SetMyCommandsConfig) (*tgbotapi.APIResponse, error) {
	return t.bot.Request(cmds)
}
func (t *Telegram) GetFile(config tgbotapi.FileConfig) (tgbotapi.File, error) {
	return t.bot.GetFile(config)
}
