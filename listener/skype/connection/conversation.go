package connection

import (
	"strings"
)

type Resource struct {
	ConversationLink      string        `json:"conversationLink"`
	Type                  string        `json:"type"`
	EventId               string        `json:"eventId"`
	From                  string        `json:"from"`
	ClientMessageId       string        `json:"clientmessageid"`
	SkypeEditedId         string        `json:"skypeeditedid"`
	Version               interface{}   `json:"version"` // string|number
	MessageType           string        `json:"messagetype"`
	CounterPartyMessageId string        `json:"counterpartymessageid"`
	ImDisplayName         string        `json:"imdisplayname"`
	Content               string        `json:"content"`
	ComposeTime           string        `json:"composetime"`
	OriginContextId       string        `json:"origincontextid"`
	OriginalArrivalTime   string        `json:"originalarrivaltime"`
	AckRequired           string        `json:"ackrequired"`
	ContentType           string        `json:"contenttype"`
	IsVideoCall           string        `json:"isVideoCall"` // "FALSE|TRUE"
	IsActive              bool          `json:"isactive"`
	ThreadTopic           string        `json:"threadtopic"`
	ContentFormat         string        `json:"contentformat"`
	ETag                  string        `json:"eTag"`
	Members               []interface{} `json:"members"`
	Id                    string        `json:"id"`
	Jid                   string        `json:"jid"`       // conversation id(custom filed)
	SendId                string        `json:"sendid"`    // send id id(custom filed)
	Timestamp             int64         `json:"timestamp"` // custom filed
	UserPresence
	EndpointPresence
	AmsReferences []string `json:"amsreferences"`
	Properties    struct {
		UrlPreviews  string   `json:"urlpreviews"`
		Capabilities []string `json:"capabilities"`
	} `json:"properties"`
}

func (r *Resource) GetFromMe(ce *Connection) bool {
	if r.ConversationLink != "" {
		ConversationLinkArr := strings.Split(r.ConversationLink, "/conversations/")
		r.Jid = ConversationLinkArr[1]
	}

	if r.From != "" {
		FromArr := strings.Split(r.From, "/contacts/")
		r.SendId = FromArr[1]
	}

	if ce.auth.profile != nil && ce.auth.profile.Username != "" && "8:"+ce.auth.profile.Username == r.SendId {
		return true
	}
	
	return false
}

type UserPresence struct {
	Id                       string   `json:"id"`
	Type                     string   `json:"type"`
	SelfLink                 string   `json:"selfLink"`
	Availability             string   `json:"availability"`
	Status                   Presence `json:"status"`
	Capabilities             string   `json:"capabilities"`
	LastSeenAt               string   `json:"lastSeenAt"`
	EndpointPresenceDocLinks []string `json:"endpointPresenceDocLinks"`
}

type EndpointPresence struct {
	Id         string `json:"id"`
	Type       string `json:"type"`
	SelfLink   string `json:"selfLink"`
	PublicInfo struct {
		Capabilities     string `json:"capabilities"`
		NodeInfo         string `json:"nodeInfo"`
		SkypeNameVersion string `json:"skypeNameVersion"`
		Typ              string `json:"typ"`
		Version          string `json:"version"`
	} `json:"publicInfo"`
	PrivateInfo struct {
		EpName string `json:"epname"`
	} `json:"privateInfo"`
}

type ThreadProperties struct {
	Topic       string `json:"topic"`
	Lastjoinat  string `json:"lastjoinat"`  // ? a timestamp ? example: "1421342788493"
	Lastleaveat string `json:"lastleaveat"` // ? a timestamp ? example: "1421342788493",a value in this field means that you have left the current session conversation
	Version     string `json:"version"`     // ? a timestamp ? example: "1464029299838"
	Members     string `json:"members"`
	Membercount string `json:"membercount"`
}

type ConversationProperties struct {
	ConversationStatusProperties string `json:"conversationstatusproperties"` // ?
	ConsumptionHorizonPublished  string `json:"consumptionhorizonpublished"`  // ?
	OneToOneThreadId             string `json:"onetoonethreadid"`             // ?
	LastImReceivedTime           string `json:"lastimreceivedtime"`           // ?
	ConsumptionHorizon           string `json:"consumptionhorizon"`           // ?
	ConversationStatus           string `json:"conversationstatus"`           // ?
	IsEmptyConversation          string `json:"isemptyconversation"`          // ?
	IsFollowed                   string `json:"isfollowed"`                   // ?
}

type LastMessage struct {
	Id                  string `json:"id"`                  // ?
	OriginContextId     string `json:"origincontextid"`     // ?
	OriginalArrivalTime string `json:"originalarrivaltime"` // ?
	MessageType         string `json:"messagetype"`         // ?
	Version             string `json:"version"`             // ?
	ComposeTime         string `json:"composetime"`         // ?
	ClientMessageId     string `json:"clientmessageid"`     // ?
	ConversationLink    string `json:"conversationLink"`    // ?
	Content             string `json:"content"`             // ?
	Type                string `json:"type"`                // ?
	ConversationId      string `json:"conversationid"`      // ?
	From                string `json:"from"`                // ?
}

type Conversation struct {
	TargetLink                string                 `json:"targetLink"`
	ResourceLink              string                 `json:"resourceLink"`
	ResourceType              string                 `json:"resourceType"`
	ThreadProperties          ThreadProperties       `json:"threadProperties"`
	Id                        interface{}            `json:"id"`      // string | int?
	Type                      string                 `json:"type"`    // "Conversation" | string;
	Version                   int64                  `json:"version"` // a timestamp ? example: 1464030261015
	Properties                ConversationProperties `json:"properties"`
	LastMessage               LastMessage            `json:"lastMessage"`
	Messages                  string                 `json:"message"`
	LastUpdatedMessageId      int64                  `json:"lastUpdatedMessageId"`
	LastUpdatedMessageVersion int64                  `json:"lastUpdatedMessageVersion"`
	Resource                  Resource               `json:"resource"`
	Time                      string                 `json:"time"`
}
