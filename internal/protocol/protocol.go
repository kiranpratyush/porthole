package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

const MaxFrameSize = 1 << 20

type MsgType string

const (
	TypeRegister   MsgType = "REGISTER"
	TypeRegisterOK MsgType = "REGISTER_OK"
	TypeProxy      MsgType = "PROXY"
	TypeProxyOK    MsgType = "PROXY_OK"
)

func (m Message) Valid() bool {
	validTypes := []MsgType{TypeRegister, TypeRegisterOK, TypeProxy, TypeProxyOK}
	return slices.Contains(validTypes, m.Type)
}

type Message struct {
	Type MsgType `json:"type"`
	ID   uint64  `json:"id,omitempty"`
}

func WriteFrame(writer io.Writer, m Message) error {
	serializedMessage, err := json.Marshal(m)
	if err != nil {
		return err
	}
	frame := make([]byte, len(serializedMessage)+4)
	binary.BigEndian.PutUint32(frame[:4], uint32(len(serializedMessage)))
	copy(frame[4:], serializedMessage)
	_, err = writer.Write(frame)
	return err
}
func ReadFrame(reader io.Reader) (Message, error) {

	var v Message
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return v, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > MaxFrameSize {
		return v, fmt.Errorf("frame too large: %d bytes", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return v, err
	}
	err := json.Unmarshal(payload, &v)
	if err != nil {
		return v, err
	}
	if !v.Valid() {
		return v, fmt.Errorf("Invalid Message Type %s", v.Type)
	}
	return v, nil
}
