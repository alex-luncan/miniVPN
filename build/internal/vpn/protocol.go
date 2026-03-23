package vpn

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// Protocol message types
const (
	MsgTypeHandshakeInit     = 0x01
	MsgTypeHandshakeResponse = 0x02
	MsgTypeHandshakeComplete = 0x03
	MsgTypeData              = 0x04
	MsgTypeKeepAlive         = 0x05
	MsgTypeDisconnect        = 0x06
)

// Protocol constants
const (
	ProtocolVersion = 1
	MaxMessageSize  = 65535
	HeaderSize      = 5 // 1 byte type + 4 bytes length
)

// Message represents a protocol message
type Message struct {
	Type    byte
	Payload []byte
}

// HandshakeInit is sent by client to initiate connection
type HandshakeInit struct {
	Version      uint8
	SecretHash   [32]byte
	ClientPubKey [32]byte
	Timestamp    int64
}

// HandshakeResponse is sent by server after validating secret
type HandshakeResponse struct {
	Version      uint8
	ServerPubKey [32]byte
	SessionID    [16]byte
	Timestamp    int64
}

// HandshakeComplete is sent by client to confirm key exchange
type HandshakeComplete struct {
	SessionID [16]byte
	Encrypted []byte // Encrypted confirmation with derived key
}

// WriteMessage writes a message to the connection
func WriteMessage(conn net.Conn, msg *Message) error {
	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Create header
	header := make([]byte, HeaderSize)
	header[0] = msg.Type
	binary.BigEndian.PutUint32(header[1:], uint32(len(msg.Payload)))

	// Write header
	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write payload
	if len(msg.Payload) > 0 {
		if _, err := conn.Write(msg.Payload); err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
	}

	return nil
}

// ReadMessage reads a message from the connection
func ReadMessage(conn net.Conn) (*Message, error) {
	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read header
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:])

	if payloadLen > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d", payloadLen)
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	return &Message{
		Type:    msgType,
		Payload: payload,
	}, nil
}

// EncodeHandshakeInit encodes a handshake init message
func EncodeHandshakeInit(init *HandshakeInit) []byte {
	buf := make([]byte, 1+32+32+8)
	buf[0] = init.Version
	copy(buf[1:33], init.SecretHash[:])
	copy(buf[33:65], init.ClientPubKey[:])
	binary.BigEndian.PutUint64(buf[65:], uint64(init.Timestamp))
	return buf
}

// DecodeHandshakeInit decodes a handshake init message
func DecodeHandshakeInit(data []byte) (*HandshakeInit, error) {
	if len(data) < 73 {
		return nil, fmt.Errorf("handshake init too short")
	}

	init := &HandshakeInit{
		Version:   data[0],
		Timestamp: int64(binary.BigEndian.Uint64(data[65:])),
	}
	copy(init.SecretHash[:], data[1:33])
	copy(init.ClientPubKey[:], data[33:65])

	return init, nil
}

// EncodeHandshakeResponse encodes a handshake response message
func EncodeHandshakeResponse(resp *HandshakeResponse) []byte {
	buf := make([]byte, 1+32+16+8)
	buf[0] = resp.Version
	copy(buf[1:33], resp.ServerPubKey[:])
	copy(buf[33:49], resp.SessionID[:])
	binary.BigEndian.PutUint64(buf[49:], uint64(resp.Timestamp))
	return buf
}

// DecodeHandshakeResponse decodes a handshake response message
func DecodeHandshakeResponse(data []byte) (*HandshakeResponse, error) {
	if len(data) < 57 {
		return nil, fmt.Errorf("handshake response too short")
	}

	resp := &HandshakeResponse{
		Version:   data[0],
		Timestamp: int64(binary.BigEndian.Uint64(data[49:])),
	}
	copy(resp.ServerPubKey[:], data[1:33])
	copy(resp.SessionID[:], data[33:49])

	return resp, nil
}

// EncodeHandshakeComplete encodes a handshake complete message
func EncodeHandshakeComplete(complete *HandshakeComplete) []byte {
	buf := make([]byte, 16+len(complete.Encrypted))
	copy(buf[:16], complete.SessionID[:])
	copy(buf[16:], complete.Encrypted)
	return buf
}

// DecodeHandshakeComplete decodes a handshake complete message
func DecodeHandshakeComplete(data []byte) (*HandshakeComplete, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("handshake complete too short")
	}

	complete := &HandshakeComplete{
		Encrypted: make([]byte, len(data)-16),
	}
	copy(complete.SessionID[:], data[:16])
	copy(complete.Encrypted, data[16:])

	return complete, nil
}
