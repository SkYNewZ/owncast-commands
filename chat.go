package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

const (
	CHAT          = "CHAT"
	PING          = "PING"
	PONG          = "PONG"
	SYSTEM        = "SYSTEM"
	NAME_CHANGE   = "NAME_CHANGE"
	requestOrigin = "http://localhost"
)

var ErrCloseConnectionTimeout = errors.New("ChatService.Close(): timeout exceeded while closing websocket connection")

// ProcessMessageFunc get a received message from chat and return the answer.
// answer can be nil.
// Function must be concurrent safe
type ProcessMessageFunc func(input *Message) *Message

// Message describe a Owncast chat message
type Message struct {
	Author    string    `json:"author,omitempty"`
	Body      string    `json:"body,omitempty"`
	ID        string    `json:"id,omitempty"`
	Type      string    `json:"type"`
	Visible   bool      `json:"visible,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// String simply print message as JSON value
func (c Message) String() string {
	data, _ := json.Marshal(c)
	return string(data)
}

// ChatService performs action on Owncast chat
type ChatService struct {
	ws      *websocket.Conn
	pingCh  chan *Message
	chatCh  chan *Message
	jobFunc ProcessMessageFunc
	doneCh  chan bool
}

// Config is required filed to initiate a websocket connection
type Config struct {
	Scheme              string
	Host                string
	Path                string
	CommandExecutorFunc ProcessMessageFunc
}

func (c *Config) validate() error {
	if c.Scheme == "" {
		return fmt.Errorf("missing websocket scheme")
	}

	if c.Host == "" {
		return fmt.Errorf("missing websocket host")
	}

	if c.CommandExecutorFunc == nil {
		return fmt.Errorf("missing command executor function")
	}

	_, err := url.Parse(fmt.Sprintf("%s://%s%s", c.Scheme, c.Host, c.Path))
	if err != nil {
		return fmt.Errorf("invalid websocket url: %v", err)
	}

	return nil
}

// NewChatService create a new websocket listener
func NewChatService(config *Config) (*ChatService, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	u := &url.URL{
		Scheme: config.Scheme,
		Host:   config.Host,
		Path:   config.Path,
	}
	log.Debugf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Origin": {requestOrigin}})
	if err != nil {
		return nil, err
	}

	return &ChatService{
		ws:      c,
		pingCh:  make(chan *Message),
		chatCh:  make(chan *Message),
		doneCh:  make(chan bool),
		jobFunc: config.CommandExecutorFunc,
	}, nil
}

// Close websocket connection
func (c *ChatService) Close(ctx context.Context) error {
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	err := c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return fmt.Errorf("ChatService.Close(): %v", err)
	}

	select {
	case <-c.doneCh:
		return nil
	case <-ctx.Done():
		return ErrCloseConnectionTimeout
	}
}

// Listen start routines to listen for input/output messages
func (c *ChatService) Listen() {
	go c.listenRead()
	go c.listenWrite()
}

func (c *ChatService) send(message *Message) error {
	return c.ws.WriteJSON(message)
}

// listenRead receive each chat messages and dispatch them to related Go channel
func (c *ChatService) listenRead() {
	defer close(c.doneCh)
	for {
		var message Message
		if err := c.ws.ReadJSON(&message); err != nil {
			// If unexpected error
			v, ok := err.(*websocket.CloseError)
			if !ok || v.Code != websocket.CloseNormalClosure {
				log.WithField("body", message.Body).WithField("type", message.Type).Errorln(err)
			}

			return
		}

		log.Debugf("Received %s request", message.Type)

		// Dispatch message
		switch message.Type {
		case PING:
			c.pingCh <- &message
		case CHAT:
			c.chatCh <- &message

		// Just ignore this kind of message
		case SYSTEM:
		case NAME_CHANGE:

		default:
			log.Warningf("unknow message type received: %s", message)
		}
	}
}

// listenWrite listen to Go channels and send back right message
func (c *ChatService) listenWrite() {
	for {
		select {
		// Stop this routine
		case <-c.doneCh:
			return

		// Listen to PING
		case <-c.pingCh:
			m := &Message{Type: PONG}
			if err := c.send(m); err != nil {
				log.WithField("body", m.Body).WithField("type", m.Type).Errorln(err)
				continue
			}

		// Listen incoming to generic message
		case input := <-c.chatCh:
			// Separate routine to process multiple messages at once
			go func(m *Message) {
				if output := c.jobFunc(m); output != nil {
					if err := c.send(output); err != nil {
						log.WithField("body", output.Body).WithField("type", output.Type).Errorln(err)
					}
				}
			}(input)
		}
	}
}
