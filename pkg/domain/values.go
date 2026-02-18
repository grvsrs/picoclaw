package domain

// ---------------------------------------------------------------------------
// Shared value objects — used across bounded contexts
// ---------------------------------------------------------------------------

// ChannelType represents the kind of messaging channel.
type ChannelType string

const (
	ChannelTelegram  ChannelType = "telegram"
	ChannelDiscord   ChannelType = "discord"
	ChannelSlack     ChannelType = "slack"
	ChannelWhatsApp  ChannelType = "whatsapp"
	ChannelFeishu    ChannelType = "feishu"
	ChannelDingTalk  ChannelType = "dingtalk"
	ChannelQQ        ChannelType = "qq"
	ChannelMaixCam   ChannelType = "maixcam"
	ChannelWeb       ChannelType = "web"
	ChannelAPI       ChannelType = "api"
	ChannelCLI       ChannelType = "cli"
)

// AllChannelTypes returns all known channel types.
func AllChannelTypes() []ChannelType {
	return []ChannelType{
		ChannelTelegram, ChannelDiscord, ChannelSlack, ChannelWhatsApp,
		ChannelFeishu, ChannelDingTalk, ChannelQQ, ChannelMaixCam,
		ChannelWeb, ChannelAPI, ChannelCLI,
	}
}

// String implements fmt.Stringer.
func (ct ChannelType) String() string { return string(ct) }

// Valid returns true if the channel type is recognized.
func (ct ChannelType) Valid() bool {
	for _, t := range AllChannelTypes() {
		if t == ct {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------

// ProviderType represents the kind of LLM provider.
type ProviderType string

const (
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderOpenAI     ProviderType = "openai"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderGroq       ProviderType = "groq"
	ProviderZhipu      ProviderType = "zhipu"
	ProviderVLLM       ProviderType = "vllm"
	ProviderGemini     ProviderType = "gemini"
	ProviderLocal      ProviderType = "local"
	ProviderMoonshot   ProviderType = "moonshot"
)

func (pt ProviderType) String() string { return string(pt) }

// ---------------------------------------------------------------------------

// MessageRole represents who sent a message in a conversation.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

func (mr MessageRole) String() string { return string(mr) }

// ---------------------------------------------------------------------------

// SkillSource indicates where a skill was loaded from.
type SkillSource string

const (
	SkillSourceBuiltin   SkillSource = "builtin"
	SkillSourceWorkspace SkillSource = "workspace"
	SkillSourceGlobal    SkillSource = "global"
	SkillSourceHub       SkillSource = "hub"
	SkillSourceGitHub    SkillSource = "github"
)

func (ss SkillSource) String() string { return string(ss) }

// ---------------------------------------------------------------------------

// ConnectionStatus represents the health state of any connectable resource.
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusError        ConnectionStatus = "error"
	StatusIdle         ConnectionStatus = "idle"
)

func (cs ConnectionStatus) String() string { return string(cs) }

// ---------------------------------------------------------------------------

// Severity classifies log and event severity.
type Severity string

const (
	SeverityDebug   Severity = "debug"
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityCritical Severity = "critical"
)

func (s Severity) String() string { return string(s) }

// ---------------------------------------------------------------------------

// Tag is a string label for categorization (skills, workflows, etc.)
type Tag string

// Tags is an ordered set of tags.
type Tags []Tag

// Contains returns true if the tag set includes the given tag.
func (t Tags) Contains(tag Tag) bool {
	for _, tt := range t {
		if tt == tag {
			return true
		}
	}
	return false
}

// Strings returns the tags as a string slice.
func (t Tags) Strings() []string {
	out := make([]string, len(t))
	for i, tt := range t {
		out[i] = string(tt)
	}
	return out
}

// ---------------------------------------------------------------------------

// Metadata is a generic key-value map for extensible properties.
type Metadata map[string]string

// Get returns a metadata value, or empty string if not present.
func (m Metadata) Get(key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

// Set writes a metadata key-value pair. Initializes the map if nil.
func (m *Metadata) Set(key, value string) {
	if *m == nil {
		*m = make(Metadata)
	}
	(*m)[key] = value
}
