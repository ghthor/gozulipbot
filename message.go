package gozulipbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// A Message is all of the necessary metadata to post on Zulip.
// It can be either a public message, where Topic is set, or a private message,
// where there is at least one element in Emails.
//
// If the length of Emails is not 0, functions will always assume it is a private message.
type Message struct {
	Stream  string
	Topic   string
	Emails  []string
	Content string
}

type EventMessage struct {
	AvatarURL        string           `json:"avatar_url"`
	Client           string           `json:"client"`
	Content          string           `json:"content"`
	ContentType      string           `json:"content_type"`
	DisplayRecipient DisplayRecipient `json:"display_recipient"`
	GravatarHash     string           `json:"gravatar_hash"`
	ID               int              `json:"id"`
	RecipientID      int              `json:"recipient_id"`
	SenderDomain     string           `json:"sender_domain"`
	SenderEmail      string           `json:"sender_email"`
	SenderFullName   string           `json:"sender_full_name"`
	SenderID         int              `json:"sender_id"`
	SenderShortName  string           `json:"sender_short_name"`
	Subject          string           `json:"subject"`
	SubjectLinks     []interface{}    `json:"subject_links"`
	Timestamp        int              `json:"timestamp"`
	Type             string           `json:"type"`
}

type DisplayRecipient struct {
	Users []User `json:"users,omitempty"`
	Topic string `json:"topic,omitempty"`
}

type User struct {
	Domain        string `json:"domain"`
	Email         string `json:"email"`
	FullName      string `json:"full_name"`
	ID            int    `json:"id"`
	IsMirrorDummy bool   `json:"is_mirror_dummy"`
	ShortName     string `json:"short_name"`
}

func (d *DisplayRecipient) UnmarshalJSON(b []byte) (err error) {
	topic, users := "", make([]User, 1)
	if err = json.Unmarshal(b, &topic); err == nil {
		d.Topic = topic
		return
	}
	if err = json.Unmarshal(b, &users); err == nil {
		d.Users = users
		return
	}
	return
}

// Message posts a message to Zulip. If any emails have been set on the message,
// the message will be re-routed to the PrivateMessage function.
func (b *Bot) Message(m Message) (*http.Response, error) {
	if m.Content == "" {
		return nil, errors.New("content cannot be empty")
	}

	// if any emails are set, this is a private message
	if len(m.Emails) != 0 {
		return b.PrivateMessage(m)
	}

	// otherwise it's a stream message
	if m.Stream == "" {
		return nil, errors.New("stream cannot be empty")
	}
	if m.Topic == "" {
		return nil, errors.New("topic cannot be empty")
	}
	req, err := b.constructMessageRequest(m)
	if err != nil {
		return nil, err
	}
	return b.client.Do(req)
}

// PrivateMessage sends a message to the users in the message email slice.
func (b *Bot) PrivateMessage(m Message) (*http.Response, error) {
	if len(m.Emails) == 0 {
		return nil, errors.New("there must be at least one recipient")
	}
	req, err := b.constructMessageRequest(m)
	if err != nil {
		return nil, err
	}

	return b.client.Do(req)
}

// Respond sends a given message as a response to whatever context from which
// an EventMessage was received.
func (b *Bot) Respond(e EventMessage, response string) (*http.Response, error) {
	if response == "" {
		return nil, errors.New("Message response cannot be blank")
	}
	m := Message{
		Stream:  e.DisplayRecipient.Topic,
		Topic:   e.Subject,
		Content: response,
	}
	if m.Topic != "" {
		return b.Message(m)
	}
	// private message
	if m.Stream == "" {
		emails, err := b.privateResponseList(e)
		if err != nil {
			return nil, err
		}
		m.Emails = emails
		return b.Message(m)
	}
	return nil, fmt.Errorf("EventMessage is not understood: %v\n", e)
}

// privateResponseList gets the list of other users in a private multiple
// message conversation.
func (b *Bot) privateResponseList(e EventMessage) ([]string, error) {
	var out []string
	for _, u := range e.DisplayRecipient.Users {
		if u.Email != b.Email {
			out = append(out, u.Email)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("EventMessage had no Users within the DisplayRecipient")
	}
	return out, nil
}

// constructMessageRequest is a helper for simplifying sending a message.
func (b *Bot) constructMessageRequest(m Message) (*http.Request, error) {
	to := m.Stream
	mtype := "stream"

	le := len(m.Emails)
	if le != 0 {
		mtype = "private"
	}
	if le == 1 {
		to = m.Emails[0]
	}
	if le > 1 {
		for i, e := range m.Emails {
			to += e
			if i != le-1 {
				to += ", "
			}
		}
	}

	values := url.Values{}
	values.Set("type", mtype)
	values.Set("to", to)
	values.Set("content", m.Content)
	if mtype == "stream" {
		values.Set("subject", m.Topic)
	}

	return b.constructRequest("POST", "messages", values.Encode())
}

func ParseEventMessages(rawEventResponse []byte) ([]EventMessage, error) {
	rawResponse := map[string]json.RawMessage{}
	err := json.Unmarshal(rawEventResponse, &rawResponse)
	if err != nil {
		return nil, err
	}

	events := []map[string]json.RawMessage{}
	err = json.Unmarshal(rawResponse["events"], &events)
	if err != nil {
		return nil, err
	}

	messages := []EventMessage{}
	for _, event := range events {
		var msg EventMessage
		err = json.Unmarshal(event["message"], &msg)
		// TODO: determine if this check should be here
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
