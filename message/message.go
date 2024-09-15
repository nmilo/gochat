package message

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Message types
const (
	MsgTypeRegister = iota
	MsgTypeChat
	MsgTypePeerConnected
	MsgTypePeerDisconnected
	MsgTypePeerHeartbeat
	MsgTypePing
	MsgTypeKeyExchange
)

// Message struct
type Message struct {
	Type         uint8  // Message type: register, chat, ping etc.
	Content      []byte // The actual message content
	ExtraContent []byte // Used for additional content
}

// Encode the Message struct to bytes
func (m *Message) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	// Write the message type
	if err := binary.Write(buf, binary.BigEndian, m.Type); err != nil {
		return nil, err
	}
	// Write the message content
	if err := binary.Write(buf, binary.BigEndian, m.Content); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode bytes into a Message struct
func Decode(data []byte) (*Message, error) {
	buf := bytes.NewBuffer(data)
	msg := &Message{}

	// Read the message type
	if err := binary.Read(buf, binary.BigEndian, &msg.Type); err != nil {
		return nil, err
	}
	// Read the message content
	msg.Content = buf.Bytes()

	return msg, nil
}

// Display the message for debugging purposes
func (m *Message) String() string {
	return fmt.Sprintf("Type: %d, Content: %s", m.Type, string(m.Content))
}
