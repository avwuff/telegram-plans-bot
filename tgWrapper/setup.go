package tgWrapper

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
	"strings"
)

type Telegram struct {
	bot *tgbotapi.BotAPI
	key string
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
	t.bot, err = tgbotapi.NewBotAPI(t.key)

	if err != nil {
		return err
	}
	// TODO remove this
	t.bot.Debug = true
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
