package model

import (
	"fmt"
)

type Alert struct {
	Channel string
	Text    string
	From    string
	Chat    string
}

func (a *Alert) String() string {
	return fmt.Sprintf("%s: %s: %s", a.Channel, a.From, a.Text)
}
