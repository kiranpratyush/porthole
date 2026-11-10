package internal

import (
	"net"
	"ngrok-clone/internal/protocol"
	"sync"
)

type Client struct {
	Con net.Conn
	mu  sync.Mutex
}

func (c *Client) Send(m protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return protocol.WriteFrame(c.Con, m)
}
