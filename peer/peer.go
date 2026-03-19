package peer

import (
	"fmt"
	"net"
	"time"
)

type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
)

type Message struct {
	ID      MessageID
	Payload []byte
}
type Connection struct {
	Conn     net.Conn
	Bitfield []byte
	PeerID   [20]byte
}

func Handshake(ip string, port uint16, infoHash [20]byte, peerID [20]byte) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	handshake := make([]byte, 68)
	handshake[0] = 19
	copy(handshake[1:20], "BitTorrent protocol")
	copy(handshake[28:48], infoHash[:])
	copy(handshake[48:68], peerID[:])

	conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.Write(handshake)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}
	response := make([]byte, 68)
	_, err = readFull(conn, response)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read handshake: %w", err)
	}
	var theirInfoHash [20]byte
	copy(theirInfoHash[:], response[28:48])
	if theirInfoHash != infoHash {
		conn.Close()
		return nil, fmt.Errorf("info hash mismatch")
	}

	var theirPeerID [20]byte
	copy(theirPeerID[:], response[48:68])

	return &Connection{
		Conn:   conn,
		PeerID: theirPeerID,
	}, nil
}

func (c *Connection) SendMessage(msg Message) error {
	buf := make([]byte, 4+1+len(msg.Payload))
	length := uint32(1 + len(msg.Payload))
	buf[0] = byte(length >> 24)
	buf[1] = byte(length >> 16)
	buf[2] = byte(length >> 8)
	buf[3] = byte(length)

	buf[4] = byte(msg.ID)
	copy(buf[5:], msg.Payload)

	_, err := c.Conn.Write(buf)
	return err
}

func (c *Connection) ReadMessage() (Message, error) {

	lenBuf := make([]byte, 4)
	_, err := readFull(c.Conn, lenBuf)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read message length: %w", err)
	}
	length := uint32(lenBuf[0])<<24 | uint32(lenBuf[1])<<16 | uint32(lenBuf[2])<<8 | uint32(lenBuf[3])
	if length == 0 {
		return Message{}, nil
	}
	msgBuf := make([]byte, length)
	_, err = readFull(c.Conn, msgBuf)
	if err != nil {
		return Message{}, fmt.Errorf("failed to read message body: %w", err)
	}

	return Message{
		ID:      MessageID(msgBuf[0]),
		Payload: msgBuf[1:],
	}, nil
}
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
