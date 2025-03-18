// backend/internal/integrations/slack/models.go
package slack

// Message represents a Slack message
type Message struct {
	Channel     string       `json:"channel,omitempty"`
	Text        string       `json:"text,omitempty"`
	Blocks      []Block      `json:"blocks,omitempty"`
	ThreadTS    string       `json:"thread_ts,omitempty"`
	TS          string       `json:"ts,omitempty"`
	AsUser      bool         `json:"as_user,omitempty"`
	Markdown    bool         `json:"mrkdwn,omitempty"`
	LinkNames   int          `json:"link_names,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Block represents a Slack block element
type Block struct {
	Type      string                 `json:"type"`
	BlockID   string                 `json:"block_id,omitempty"`
	Text      *TextObject            `json:"text,omitempty"`
	Fields    []*TextObject          `json:"fields,omitempty"`
	Accessory map[string]interface{} `json:"accessory,omitempty"`
	Elements  []interface{}          `json:"elements,omitempty"`
}

// TextObject represents a Slack text object
type TextObject struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// NewTextObject creates a new text object
func NewTextObject(type_, text string, emoji bool) *TextObject {
	return &TextObject{
		Type:  type_,
		Text:  text,
		Emoji: emoji,
	}
}

// Attachment represents a Slack message attachment
type Attachment struct {
	Fallback   string            `json:"fallback,omitempty"`
	Color      string            `json:"color,omitempty"`
	Pretext    string            `json:"pretext,omitempty"`
	AuthorName string            `json:"author_name,omitempty"`
	AuthorLink string            `json:"author_link,omitempty"`
	AuthorIcon string            `json:"author_icon,omitempty"`
	Title      string            `json:"title,omitempty"`
	TitleLink  string            `json:"title_link,omitempty"`
	Text       string            `json:"text,omitempty"`
	Fields     []AttachmentField `json:"fields,omitempty"`
	ImageURL   string            `json:"image_url,omitempty"`
	ThumbURL   string            `json:"thumb_url,omitempty"`
	Footer     string            `json:"footer,omitempty"`
	FooterIcon string            `json:"footer_icon,omitempty"`
	Timestamp  int64             `json:"ts,omitempty"`
	Actions    []Action          `json:"actions,omitempty"`
}

// AttachmentField represents a field in a Slack attachment
type AttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Action represents a button or menu in a Slack attachment
type Action struct {
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	URL     string   `json:"url,omitempty"`
	Style   string   `json:"style,omitempty"`
	Name    string   `json:"name"`
	Value   string   `json:"value"`
	Options []Option `json:"options,omitempty"`
}

// Option represents an option in a Slack select menu
type Option struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// Command represents a Slack slash command
type Command struct {
	Token          string `form:"token"`
	TeamID         string `form:"team_id"`
	TeamDomain     string `form:"team_domain"`
	EnterpriseID   string `form:"enterprise_id"`
	EnterpriseName string `form:"enterprise_name"`
	ChannelID      string `form:"channel_id"`
	ChannelName    string `form:"channel_name"`
	UserID         string `form:"user_id"`
	UserName       string `form:"user_name"`
	Command        string `form:"command"`
	Text           string `form:"text"`
	ResponseURL    string `form:"response_url"`
	TriggerID      string `form:"trigger_id"`
}

// InteractionPayload represents a payload from a Slack interactive component
type InteractionPayload struct {
	Type        string                 `json:"type"`
	TeamID      string                 `json:"team_id"`
	TeamDomain  string                 `json:"team_domain"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	UserID      string                 `json:"user_id"`
	UserName    string                 `json:"user_name"`
	ActionTS    string                 `json:"action_ts"`
	MessageTS   string                 `json:"message_ts"`
	CallbackID  string                 `json:"callback_id"`
	Actions     []map[string]string    `json:"actions"`
	State       string                 `json:"state"`
	ResponseURL string                 `json:"response_url"`
	Container   map[string]interface{} `json:"container"`
}

// ChannelMapping maps GRC categories to Slack channels
var ChannelMapping = map[string]string{
	"risk-management": "risk-management",
	"compliance":      "compliance-team",
	"incident":        "incident-response",
	"audit":           "audit-team",
	"vendor-risk":     "vendor-risk",
	"regulatory":      "regulatory-updates",
	"reports":         "grc-reports",
	"control-testing": "control-testing",
}
