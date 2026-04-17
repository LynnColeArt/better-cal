package httpapi

type envelope struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  *err   `json:"error,omitempty"`
}

type err struct {
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}
