package discordbot

// Request enum definitions
const (
	RequestTypePing               int32 = 1
	RequestTypeApplicationCommand int32 = 2

	RequestDataTypeChatInput      int32 = 1
)

type Request struct {
	Type  int32       `json:"type"`
	Token string      `json:"token"`
	Data  requestData `json:"data"`
}

type requestData struct {
	Type int32  `json:"type"`
	Name string `json:"name"`
}

// Response enum definitions
const (
	ResponseTypePong                   int32 = 1
	ResponseTypeChannelMessage         int32 = 4
	ResponseTypeDeferredChannelMessage int32 = 5
)

type Response struct {
	Type int32        `json:"type"`
	Data responseData `json:"data"`
}

type responseData struct {
	Content string `json:"content"`
}