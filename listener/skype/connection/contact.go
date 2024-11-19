package connection

type Presence string

const (
	PresenceOnline  Presence = "Online"
	PresenceOffline Presence = "Offline"
	PresenceIdle    Presence = "Idle"
	PresenceAway    Presence = "Away"
	PresenceHidden  Presence = "Hidden"
)
