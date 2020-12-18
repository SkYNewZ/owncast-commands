package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// ServerURL Owncast instance URL
	ServerURL = "https://stream.skynewz.dev"

	// BotName Bot messages author
	BotName = "Bot"
)

var (
	commandRegexp = regexp.MustCompile(`^<p>(?P<command>![a-z]+)<\/p>$`)
	commands      []*Command
)

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: true,
	})

	if l, err := log.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
		log.SetLevel(l)
	}
}

func main() {
	var commandFileName string
	flag.StringVar(&commandFileName, "commands-file", "commands.yml", "Describe your commands in this file")
	flag.Parse()

	// Read commands
	var err error
	commands, err = ReadCommandsFromFile(commandFileName)
	if err != nil {
		log.Fatalln(err)
	}

	// Create our chat service with the command parser function
	chatService, err := NewChatService(&Config{
		Scheme:              "wss",
		Host:                "stream.skynewz.dev",
		Path:                "/entry",
		CommandExecutorFunc: processMessageCommand,
	})
	if err != nil {
		log.Fatalln(err)
	}

	// Start listening to chat messages
	chatService.Listen()

	// Get the ctrl+C signal to safely close this program
	var quit = make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := chatService.Close(ctx); err != nil {
		log.Fatalln(err)
	}
}

// processMessageCommand check if given message is a command and perform associated action
func processMessageCommand(input *Message) *Message {
	log.WithField("id", input.ID).Tracef("Received message %s", input.Body)

	// Ensure this is a command message
	if !commandRegexp.MatchString(input.Body) {
		log.Tracef("message %q is not a command: %q", input.ID, input.Body)
		return nil
	}

	// Get command
	res := parseGroupsRegexp(commandRegexp, input.Body)
	v, ok := res["command"]
	if !ok {
		log.Errorf("command not found in message %q. It should do", input.Body)
		return nil
	}

	// Is it a existent command ?
	var command *Command
	for _, c := range commands {
		if c.Trigger == v {
			command = c
			break
		}
	}

	if command == nil {
		log.Tracef("%q: command not found", v)
		return nil
	}

	log.Debugf("running command %q", v)
	r, err := command.Parse()
	if err != nil {
		log.Errorln(err)
		return nil
	}

	// Send command result
	return &Message{
		Author: BotName,
		Body:   fmt.Sprintf("@%s %s", input.Author, r),
		Type:   CHAT,
	}
}

// parseGroupsRegexp return a map contains group keys and values from the given pattern
// https://stackoverflow.com/a/39635221
func parseGroupsRegexp(re *regexp.Regexp, v string) (r map[string]string) {
	match := re.FindStringSubmatch(v)
	r = make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i > 0 && i <= len(match) {
			r[name] = match[i]
		}
	}
	return
}
