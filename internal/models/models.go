package models

type AddQuoteRequest struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

type AddQuoteResponse struct {
	Status string `json:"status"`
	ID     int64  `json:"id"`
	Text   string `json:"text"`
	Author string `json:"author"`
}

type ErrorResponse struct {
	Status string   `json:"status"`
	Error  string   `json:"error"`
	Fields []string `json:"fields,omitempty"`
}

type SuccessDataResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type GenericMessageResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Quote struct {
	ID     int64  `json:"id"`
	Text   string `json:"text"`
	Author string `json:"author"`
}