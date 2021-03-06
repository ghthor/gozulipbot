package gozulipbot

import (
	"net/http"
	"testing"
)

func TestBot_Init(t *testing.T) {
	bot := Bot{}
	bot.Init()

	if bot.Client == nil {
		t.Error("expected bot to have client")
	}
}

func getTestBot() *Bot {
	return &Bot{
		Email:   "testbot@example.com",
		APIKey:  "apikey",
		Streams: []string{"stream a", "test bots"},
		Client:  &testClient{},
	}
}

type testClient struct {
	Request  *http.Request
	Response *http.Response
}

func (t *testClient) Do(r *http.Request) (*http.Response, error) {
	t.Request = r
	return t.Response, nil
}
