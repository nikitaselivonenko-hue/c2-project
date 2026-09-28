package service

import (
	"errors"
	"testing"

	"c2project/internal/crypto"
	"c2project/internal/protocol"
	"c2project/internal/storage"
)

func TestCreateTaskUnknownClient(t *testing.T) {
	svc := New(storage.New())
	_, err := svc.CreateTask("missing", "whoami")
	if !errors.Is(err, storage.ErrClientNotFound) {
		t.Fatalf("got %v, want ErrClientNotFound", err)
	}
}

func TestCreateAndGetTask(t *testing.T) {
	store := storage.New()
	svc := New(store)
	svc.RegisterClient("c1", "host1", "linux")

	taskID, err := svc.CreateTask("c1", "whoami")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	task := svc.GetTaskForClient("c1")
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.ID != taskID {
		t.Errorf("task ID: got %s, want %s", task.ID, taskID)
	}
	// Команда должна быть расшифрована ключом KeyC2ToClient
	plain, err := crypto.Decrypt(protocol.KeyC2ToClient, task.Command)
	if err != nil {
		t.Fatalf("decrypt command: %v", err)
	}
	if string(plain) != "whoami" {
		t.Errorf("command: got %q, want %q", plain, "whoami")
	}
}

func TestStoreResultAndGetStatus(t *testing.T) {
	store := storage.New()
	svc := New(store)
	svc.RegisterClient("c1", "host1", "linux")

	taskID, _ := svc.CreateTask("c1", "whoami")
	svc.GetTaskForClient("c1") // извлекаем из очереди

	// Клиент отправляет результат, зашифрованный вторым ключом
	encResult, _ := crypto.Encrypt(protocol.KeyC2ToClient, []byte("test\\user"))
	if err := svc.StoreResult("c1", encResult); err != nil {
		t.Fatalf("StoreResult: %v", err)
	}

	status, err := svc.GetStatus(taskID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Status != protocol.StatusDone {
		t.Errorf("status: got %s, want %s", status.Status, protocol.StatusDone)
	}
	if status.Result == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestStoreResultNoPendingTask(t *testing.T) {
	svc := New(storage.New())
	svc.RegisterClient("c1", "host1", "linux")
	err := svc.StoreResult("c1", []byte("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListClients(t *testing.T) {
	svc := New(storage.New())
	svc.RegisterClient("c1", "h1", "linux")
	svc.RegisterClient("c2", "h2", "windows")
	list := svc.ListClients()
	if len(list) != 2 {
		t.Fatalf("got %d, want 2", len(list))
	}
}