package tgWrapper

import (
	"errors"
	"net/http"
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:FAKETOKEN"

var errNoNetwork = errors.New("connection reset")

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errNoNetwork
}

func Test_networkErrorsDontContainTheToken(t *testing.T) {
	client := tokenSafeClient{client: &http.Client{Transport: failingTransport{}}}

	// This fails on its first request, to get the bot's details.
	_, err := tgbotapi.NewBotAPIWithOptions(testToken, tgbotapi.WithHTTPClient(client))

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "FAKETOKEN")
	assert.ErrorIs(t, err, errNoNetwork)
}

func Test_withoutTheSafeClientTheTokenLeaks(t *testing.T) {
	// If the library stops including the URL, tokenSafeClient can be removed.
	_, err := tgbotapi.NewBotAPIWithOptions(testToken, tgbotapi.WithHTTPClient(&http.Client{Transport: failingTransport{}}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FAKETOKEN")
}
