package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"log"

	"github.com/kyokomi/emoji"
	"gopkg.in/yaml.v2"
)

type commandFile struct {
	File []*Command `yaml:"commands"`
}

// ReadCommandsFromFile read and parse the given YAML file
func ReadCommandsFromFile(file string) ([]*Command, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var f commandFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		log.Fatalf("error: %v", err)
	}

	return f.File, nil
}

// Command describe a Owncast stream command
type Command struct {
	Trigger  string `yaml:"trigger"`
	Template string `yaml:"template"`
}

// Parse the current command template and replace placeholders with their respective result
func (c *Command) Parse() (string, error) {
	tpl := template.Must(template.New("").Funcs(template.FuncMap{
		"uptime": Uptime,
	}).Parse(c.Template))

	// Execute template functions
	var content bytes.Buffer
	if err := tpl.Execute(&content, nil); err != nil {
		return "", err
	}

	// Set emojis
	return emoji.Sprint(content.String()), nil
}
