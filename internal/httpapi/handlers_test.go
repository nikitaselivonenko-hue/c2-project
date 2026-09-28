package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"c2project/internal/crypto"
	"c2project/internal/protocol"
	"c2project/internal/service"
	"c2project/internal/storage"
)

// newTestHandler создаёт Handler поверх чистого хранилища.
// Не трогает сеть и не поднимает реальный HTTP-сервер.
func newTestHandler() (*Handler, *service.Service) {
	store := storage.New()
	svc := service.New(store)
	return NewHandler(svc), svc
}

func TestSubmitMethodNotAllowed(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/submit", nil)
	rr := httptest.NewRecorder()
	h.Submit(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestSubmitInvalidJSON(t *testing.T) {
	h, _ := newTestHandler()
	body := bytes.NewBufferString("not-json")
	req := httptest.NewRequest(http.MethodPost, "/api/submit", body)
	rr := httptest.NewRecorder()
	h.Submit(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSubmitMissingClientID(t *testing.T) {
	h, _ := newTestHandler()
	body, _ := json.Marshal(protocol.SubmitRequest{ClientID: "", Command: "abc"})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Submit(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSubmitMissingCommand(t *testing.T) {
	h, _ := newTestHandler()
	body, _ := json.Marshal(protocol.SubmitRequest{ClientID: "c1", Command: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Submit(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSubmitUnknownClient(t *testing.T) {
	h, _ := newTestHandler()

	// Корректно шифруем команду первым ключом, чтобы дойти до проверки клиента
	encCommand, err := crypto.Encrypt(protocol.KeyTerminalToC2, []byte("whoami"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	body, _ := json.Marshal(protocol.SubmitRequest{
		ClientID: "missing",
		Command:  base64.StdEncoding.EncodeToString(encCommand),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Submit(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want %d, body: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestSubmitSuccess(t *testing.T) {
	h, svc := newTestHandler()
	svc.RegisterClient("c1", "host1", "linux")

	encCommand, err := crypto.Encrypt(protocol.KeyTerminalToC2, []byte("whoami"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	body, _ := json.Marshal(protocol.SubmitRequest{
		ClientID: "c1",
		Command:  base64.StdEncoding.EncodeToString(encCommand),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Submit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp protocol.SubmitResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TaskID == "" {
		t.Fatal("expected non-empty task id")
	}
}

func TestStatusMissingID(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestStatusTaskNotFound(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/status?id=missing", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestClientsEmpty(t *testing.T) {
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rr := httptest.NewRecorder()
	h.Clients(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rr.Code, http.StatusOK)
	}
	var list []protocol.ClientInfo
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("got %d clients, want 0", len(list))
	}
}

func TestClientsWithData(t *testing.T) {
	h, svc := newTestHandler()
	svc.RegisterClient("c1", "host1", "linux")
	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rr := httptest.NewRecorder()
	h.Clients(rr, req)
	var list []protocol.ClientInfo
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d clients, want 1", len(list))
	}
	if list[0].ClientID != "c1" {
		t.Errorf("got %s, want c1", list[0].ClientID)
	}
}