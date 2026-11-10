package main

import (
	"log"
	"net"
	"ngrok-clone/internal"
	"ngrok-clone/internal/protocol"
	"sync"
	"sync/atomic"
)

const (
	ControlAddr   = ":8081"
	HostAddr      = ":8080"
	DefaultClient = "localhost"
)

type registry struct {
	clients map[string]*internal.Client
	mu      sync.Mutex
}
type pendingMap struct {
	mu sync.Mutex
	m  map[uint64]chan net.Conn
}
type server struct {
	clients  *registry
	pendings *pendingMap
	nextID   atomic.Uint32
}

func (r *registry) get(client string) *internal.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clients[client]
}

func (r *registry) register(name string, c *internal.Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, taken := r.clients[name]; taken {
		return false
	}
	r.clients[name] = c
	return true
}
func (r *registry) remove(name string, c *internal.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clients[name] == c {
		delete(r.clients, name)
	}
}

func (p *pendingMap) add(id uint64) chan net.Conn {
	ch := make(chan net.Conn, 1)
	p.mu.Lock()
	p.m[id] = ch
	p.mu.Unlock()
	return ch
}
func (p *pendingMap) remove(id uint64) (chan net.Conn, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch, ok := p.m[id]
	if ok {
		delete(p.m, id)
	}
	return ch, ok
}

func (s *server) initiateControlListener() {
	controlListener, err := net.Listen("tcp", ControlAddr)
	if err != nil {
		log.Fatal("Failed to start controller listener with error", err)
	}
	defer controlListener.Close()

	for {
		con, err := controlListener.Accept()
		if err != nil {
			log.Println("Failed to accept a connection", con)
			continue
		}
		go s.handleControlConnection(con)
	}
}

func (s *server) handleRegisterLoop(con net.Conn) {
	defer con.Close()

	c := &internal.Client{Con: con}
	if !s.clients.register(DefaultClient, c) {
		log.Println("a client is already registered, rejecting", con.RemoteAddr())
		return
	}
	defer s.clients.remove(DefaultClient, c) // runs before Close (defers are LIFO)

	if err := c.Send(protocol.Message{Type: protocol.TypeRegisterOK}); err != nil {
		log.Println("sending REGISTER_OK:", err)
		return
	}
	log.Println("client registered:", con.RemoteAddr())
	for {
		m, err := protocol.ReadFrame(con)
		if err != nil {
			log.Println("client disconnected:", err)
			return
		}
		log.Println("unexpected message on control conn:", m.Type)
	}
}

func (s *server) handleProxyOk(con net.Conn, id uint64) {
	ch, ok := s.pendings.remove(id)
	if !ok {
		log.Println("no pending user for id", id, "(late or unknown), closing")
		con.Close()
		return
	}
	ch <- con
}

func (s *server) handleControlConnection(con net.Conn) {
	m, err := protocol.ReadFrame(con)
	if err != nil {
		con.Close()
		return
	}
	switch m.Type {
	case protocol.TypeRegister:
		s.handleRegisterLoop(con)
	case protocol.TypeProxyOK:
		s.handleProxyOk(con, m.ID)
	default:
		con.Close()
		return
	}
}

func main() {
	server := &server{
		clients: &registry{clients: make(map[string]*internal.Client)},
	}
	hostListener, err := net.Listen("tcp", HostAddr)
	if err != nil {
		log.Fatal("host listener: ", err)
	}
	defer hostListener.Close()

	server.initiateControlListener()
}

/* Write everything you need to do for server part

I have to handle user
I have to listen on the host address
When a user connects get the connection
increment the connection id in the server
create a channel for that connection and put it in the pendings
send the PROXY message to the client
using select wait on the channel on two cases
chan and on a timeout
if a chan is hit then we get the conn pipe the conn with the current conn

if the timeout occurs : try to delete the channel from the pending
if the deletion is successful, then close the user connection
if it is not successful, then the conn is available in the channel
use that conn and pipe it
then the session ends*/
