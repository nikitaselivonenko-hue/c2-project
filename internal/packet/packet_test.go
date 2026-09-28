package packet

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	original := &Packet{
		Command:   CmdRegister,
		Status:    0,
		SessionID: 42,
		MessageID: 12345,
		TreeID:    7,
		Payload:   []byte("hello, packet!"),
	}
	raw := original.Serialize()
	if len(raw) != SMB2HeaderSize+len(original.Payload) {
		t.Fatalf("unexpected raw length: got %d, want %d",
			len(raw), SMB2HeaderSize+len(original.Payload))
	}
	parsed, err := Deserialize(raw)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if parsed.Command != original.Command {
		t.Errorf("Command: got %x, want %x", parsed.Command, original.Command)
	}
	if parsed.SessionID != original.SessionID {
		t.Errorf("SessionID: got %d, want %d", parsed.SessionID, original.SessionID)
	}
	if parsed.MessageID != original.MessageID {
		t.Errorf("MessageID: got %d, want %d", parsed.MessageID, original.MessageID)
	}
	if parsed.TreeID != original.TreeID {
		t.Errorf("TreeID: got %d, want %d", parsed.TreeID, original.TreeID)
	}
	if !bytes.Equal(parsed.Payload, original.Payload) {
		t.Errorf("Payload: got %q, want %q", parsed.Payload, original.Payload)
	}
}

func TestSerializeHasSMB2Marker(t *testing.T) {
	p := &Packet{Command: CmdGetTask}
	raw := p.Serialize()
	if string(raw[0:4]) != SMB2ProtocolID {
		t.Fatalf("expected marker %q, got %q", SMB2ProtocolID, raw[0:4])
	}
}

func TestDeserializeTooShort(t *testing.T) {
	_, err := Deserialize(make([]byte, 10))
	if !errors.Is(err, ErrPacketTooShort) {
		t.Fatalf("got %v, want ErrPacketTooShort", err)
	}
}

func TestDeserializeInvalidProtocol(t *testing.T) {
	data := make([]byte, SMB2HeaderSize)
	copy(data, "XXXX")
	_, err := Deserialize(data)
	if !errors.Is(err, ErrInvalidProtocol) {
		t.Fatalf("got %v, want ErrInvalidProtocol", err)
	}
}

func TestReadWriteOverTCP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	original := &Packet{
		Command:   CmdSendResult,
		MessageID: 999,
		Payload:   []byte("result data"),
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- Write(client, original)
	}()

	_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
	parsed, err := Read(server)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Write: %v", err)
	}
	if parsed.Command != original.Command {
		t.Errorf("Command: got %x, want %x", parsed.Command, original.Command)
	}
	if !bytes.Equal(parsed.Payload, original.Payload) {
		t.Errorf("Payload: got %q, want %q", parsed.Payload, original.Payload)
	}
}