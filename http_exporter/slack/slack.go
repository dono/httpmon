package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type SlackClient struct {
	WebhookURL string
	Payload    Payload
}

type Payload struct {
	Channel     string       `json:"channel"`
	Username    string       `json:"username"`
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	Title   string `json:"title"`
	Pretext string `json:"pretext"`
	Text    string `json:"text"`
	Color   string `json:"color"`
}

func NewSlack(webhookURL, channel, username string) (*SlackClient, error) {
	// URL validation
	if _, err := url.ParseRequestURI(webhookURL); err != nil {
		return nil, err
	}

	return &SlackClient{
		WebhookURL: webhookURL,
		Payload: Payload{
			Channel:  channel,
			Username: username,
		},
	}, nil
}

func (s *SlackClient) Post(title, pretext, text, color string) error {
	attachments := []Attachment{{
		Title:   title,
		Pretext: pretext,
		Text:    text,
		Color:   color,
	}}
	s.Payload.Attachments = attachments
	body, err := json.Marshal(s.Payload)
	if err != nil {
		return err
	}
	res, err := http.PostForm(s.WebhookURL, url.Values{"payload": {string(body)}})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook request error: %s", res.Status)
	}

	return nil
}
