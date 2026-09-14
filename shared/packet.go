package shared

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

const (
	SMB2ProtocolID = "\xfeSMB"
	SMB2HeaderSize = 64

	CmdRegister    = 0x0000
	CmdRegisterAck = 0x0001
	CmdGetTask     = 0x0002
	CmdTaskData    = 0x0003
	CmdSendResult  = 0x0004
	CmdResultAck   = 0x0005

	MaxPayloadSize = 16 * 1024 * 1024
)

type Packet struct {
	Command   uint16
	Status    uint32
	SessionID uint64
	MessageID uint64
	TreeID    uint32
	Payload   []byte
}

func (p *Packet) Serialize() []byte {
	buf := new(bytes.Buffer)

	buf.WriteString(SMB2ProtocolID)
	binary.Write(buf, binary.LittleEndian, uint16(SMB2HeaderSize))
	binary.Write(buf, binary.LittleEndian, uint16(0))
	binary.Write(buf, binary.LittleEndian, p.Status)
	binary.Write(buf, binary.LittleEndian, p.Command)
	binary.Write(buf, binary.LittleEndian, uint16(0))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, p.MessageID)
	binary.Write(buf, binary.LittleEndian, uint32(len(p.Payload)))
	binary.Write(buf, binary.LittleEndian, p.TreeID)
	binary.Write(buf, binary.LittleEndian, p.SessionID)
	buf.Write(make([]byte, 16))

	buf.Write(p.Payload)

	return buf.Bytes()
}

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