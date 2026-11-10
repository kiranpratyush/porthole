package main

import (
	"fmt"
	"log"
	"net"
	"ngrok-clone/internal/protocol"
	"ngrok-clone/internal/tunnel"
	"time"
)

const ControlAddr = ":8081"
const AppAddr = ":3000"
const ClientDomainName = "abc"

func connectAndRegister() (net.Conn, error) {
	conn, err := net.Dial("tcp", "localhost"+ControlAddr)
	if err != nil {
		return nil, err
	}
	if err := protocol.WriteFrame(conn, protocol.Message{Type: protocol.TypeRegister, Name: ClientDomainName}); err != nil {
		conn.Close()
		return nil, err
	}
	msg, err := protocol.ReadFrame(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if msg.Type != protocol.TypeRegisterOK {
		conn.Close()
		return nil, fmt.Errorf("expected REGISTER_OK, got %q", msg.Type)
	}
	return conn, nil
}

func handleProxyConnection(id uint64) {
	dataConn, err := net.Dial("tcp", "localhost"+ControlAddr)
	if err != nil {
		log.Println("dial-back failed for id", id, ":", err)
		return // nothing opened, nothing to close
	}

	if err := protocol.WriteFrame(dataConn, protocol.Message{Type: protocol.TypeProxyOK, ID: id}); err != nil {
		log.Println("sending PROXY_OK for id", id, ":", err)
		dataConn.Close()
		return
	}

	appConn, err := net.Dial("tcp", "localhost"+AppAddr)
	if err != nil {
		log.Println("app unreachable for id", id, ":", err)
		dataConn.Close() // server's pipe ends, user sees the close at once
		return
	}

	tunnel.Pipe(dataConn, appConn)
}
func handleReadLoop(conn net.Conn) {
	defer conn.Close()
	for {
		msg, err := protocol.ReadFrame(conn)
		if err != nil {
			return
		}
		if msg.Type == protocol.TypeProxy {
			go handleProxyConnection(msg.ID)
		}
	}
}

func main() {

	for {
		conn, err := connectAndRegister()
		if err != nil {
			time.Sleep(time.Second * 2)
			continue
		}
		handleReadLoop(conn)
	}

}
