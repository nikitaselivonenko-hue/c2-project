package storage

import (
	"errors"
	"testing"
)

func TestUpsertAndGetClient(t *testing.T) {
	s := New()
	s.UpsertClient("client-1", "host1", "linux")
	c, err := s.GetClient("client-1")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if c.Hostname != "host1" || c.OS != "linux" {
		t.Errorf("unexpected client data: %+v", c)
	}
	// Повторный upsert обновляет данные
	s.UpsertClient("client-1", "host1-updated", "windows")
	c, _ = s.GetClient("client-1")
	if c.Hostname != "host1-updated" || c.OS != "windows" {
		t.Errorf("client not updated: %+v", c)
	}
}

func TestGetClientNotFound(t *testing.T) {
	s := New()
	_, err := s.GetClient("missing")
	if !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("got %v, want ErrClientNotFound", err)
	}
}

func TestListClients(t *testing.T) {
	s := New()
	s.UpsertClient("c1", "h1", "linux")
	s.UpsertClient("c2", "h2", "windows")
	list := s.ListClients()
	if len(list) != 2 {
		t.Fatalf("got %d clients, want 2", len(list))
	}
}

func TestTaskQueueOrder(t *testing.T) {
	s := New()
	s.UpsertClient("c1", "h1", "linux")
	s.AddTask(&Task{ID: "t1", ClientID: "c1", Status: "pending"})
	s.AddTask(&Task{ID: "t2", ClientID: "c1", Status: "pending"})

	first := s.PopTask("c1")
	if first == nil || first.ID != "t1" {
		t.Fatalf("expected t1, got %+v", first)
	}
	second := s.PopTask("c1")
	if second == nil || second.ID != "t2" {
		t.Fatalf("expected t2, got %+v", second)
	}
	third := s.PopTask("c1")
	if third != nil {
		t.Fatalf("expected empty queue, got %+v", third)
	}
}

func TestSetTaskResult(t *testing.T) {
	s := New()
	s.AddTask(&Task{ID: "t1", ClientID: "c1", Status: "pending"})
	if err := s.SetTaskResult("t1", []byte("done"), "done"); err != nil {
		t.Fatalf("SetTaskResult: %v", err)
	}
	task, _ := s.GetTask("t1")
	if task.Status != "done" || string(task.Result) != "done" {
		t.Errorf("unexpected task: %+v", task)
	}
}

func TestSetTaskResultNotFound(t *testing.T) {
	s := New()
	err := s.SetTaskResult("missing", nil, "done")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("got %v, want ErrTaskNotFound", err)
	}
}

func TestFindPendingTaskByClient(t *testing.T) {
	s := New()
	s.AddTask(&Task{ID: "t1", ClientID: "c1", Status: "done"})
	s.AddTask(&Task{ID: "t2", ClientID: "c1", Status: "pending"})
	found := s.FindPendingTaskByClient("c1")
	if found == nil || found.ID != "t2" {
		t.Fatalf("expected t2, got %+v", found)
	}
}