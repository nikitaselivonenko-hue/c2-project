// Package storage реализует потокобезопасное in-memory хранилище
// клиентов, задач и очередей. Базы данных не используются.
package storage

import (
	"errors"
	"sync"
	"time"
)

// Ошибки хранилища.
var (
	ErrClientNotFound = errors.New("client not found")
	ErrTaskNotFound   = errors.New("task not found")
)

// Client описывает зарегистрированного клиента.
type Client struct {
	ID       string
	Hostname string
	OS       string
	LastSeen time.Time
}

// Task описывает задачу и её результат.
type Task struct {
	ID       string
	ClientID string
	Command  []byte
	Result   []byte
	Status   string
}

// Storage — потокобезопасное хранилище.
type Storage struct {
	mu      sync.RWMutex
	clients map[string]*Client
	tasks   map[string]*Task
	queues  map[string][]string
}

// New создаёт новое хранилище.
func New() *Storage {
	return &Storage{
		clients: make(map[string]*Client),
		tasks:   make(map[string]*Task),
		queues:  make(map[string][]string),
	}
}

// UpsertClient добавляет нового или обновляет существующего клиента.
func (s *Storage) UpsertClient(id, hostname, os string) *Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[id]
	if !ok {
		c = &Client{ID: id}
		s.clients[id] = c
	}
	if hostname != "" {
		c.Hostname = hostname
	}
	if os != "" {
		c.OS = os
	}
	c.LastSeen = time.Now()
	return c
}

// GetClient возвращает клиента по ID.
func (s *Storage) GetClient(id string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[id]
	if !ok {
		return nil, ErrClientNotFound
	}
	return c, nil
}

// ListClients возвращает срез всех клиентов.
func (s *Storage) ListClients() []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Client, 0, len(s.clients))
	for _, c := range s.clients {
		list = append(list, c)
	}
	return list
}

// AddTask добавляет задачу и ставит её в очередь клиента.
func (s *Storage) AddTask(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	s.queues[task.ClientID] = append(s.queues[task.ClientID], task.ID)
}

// GetTask возвращает задачу по ID.
func (s *Storage) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

// PopTask извлекает первую задачу из очереди клиента.
// Возвращает nil, если очередь пуста.
func (s *Storage) PopTask(clientID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.queues[clientID]
	if len(queue) == 0 {
		return nil
	}
	taskID := queue[0]
	s.queues[clientID] = queue[1:]
	return s.tasks[taskID]
}

// SetTaskResult сохраняет результат и обновляет статус задачи.
func (s *Storage) SetTaskResult(taskID string, result []byte, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	t.Result = result
	t.Status = status
	return nil
}

// FindPendingTaskByClient возвращает первую pending-задачу для клиента.
func (s *Storage) FindPendingTaskByClient(clientID string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.ClientID == clientID && t.Status == "pending" {
			return t
		}
	}
	return nil
}