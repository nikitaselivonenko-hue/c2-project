package shared

var (
	KeyTerminalToC2 = []byte("0123456789abcdef0123456789abcdef") // 32 байта
	KeyC2ToClient   = []byte("fedcba9876543210fedcba9876543210") // 32 байта
)

const (
	EndpointSubmit = "/api/submit" // POST: отправить команду
	EndpointStatus = "/api/status" // GET: узнать статус задачи по id
	EndpointList   = "/api/clients" // GET: список клиентов

	StatusPending = "pending"
	StatusDone    = "done"
	StatusError   = "error"
)

type SubmitRequest struct {
	ClientID string `json:"client_id"`
	Command  string `json:"command"`
}

type SubmitResponse struct {
	TaskID string `json:"task_id"`
}

type StatusResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

type ClientInfo struct {
	ClientID  string `json:"client_id"`
	Hostname  string `json:"hostname"`
	OS        string `json:"os"`
	LastSeen  string `json:"last_seen"`
}