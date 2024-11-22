package model

import (
	"strings"
)

type Alert struct {
	Channel string
	Text    string
	From    string
	Chat    string
}

func (a *Alert) String() string {
	return strings.Join([]string{a.Channel, a.From, a.Text}, ": ")
}
