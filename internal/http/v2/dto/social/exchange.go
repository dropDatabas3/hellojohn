package social

type ExchangeRequest struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
}

type ExchangeResponse struct {
	OK bool `json:"ok"`
}
