package server

const (
	ContentTypeJSON = "application/json"
	ContentTypeHTML = "text/html; charset=utf-8"
	ContentTypeText = "text/plain; charset=utf-8"
)

type JSONErrorType string

const (
	ErrorTypeApplication JSONErrorType = "application"
	ErrorTypeServer      JSONErrorType = "server"
)

type JSONResponse struct {
	Status    int
	Data      any
	ErrorType JSONErrorType
	Error     map[string]any
}
