package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"ngrok-clone/internal"
	"ngrok-clone/internal/protocol"
	"ngrok-clone/internal/tunnel"
	"ngrok-clone/internal/utils"
	"sync"
	"sync/atomic"
	"time"
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
	nextID   atomic.Uint64
}

func (r *registry) get(client string) *internal.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clients[client]
}

func (r *registry) register(name string, c *internal.Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(name) == 0 {
		return false
	}
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

func serve(name string, l net.Listener, handle func(net.Conn)) error {
	for {
		con, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return fmt.Errorf("%s listener closed: %w", name, err)
			}
			log.Printf("%s: accept failed: %v", name, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go handle(con)
	}
}

func (s *server) handleHostConnection(con net.Conn) {
	/* Here extract the header and get the client based on subdomain
	 */
	var buf bytes.Buffer
	teeReader := bufio.NewReader(io.TeeReader(con, &buf))
	req, err := http.ReadRequest(teeReader)
	if err != nil {
		log.Print("Could not parse request", err)
		con.Close()
		return
	}
	/* Extract the domain name here */
	host := req.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	clientName, err := utils.ParseClientName(host)

	if err != nil {
		log.Printf("Failed to get subdomain from host %s with error %v", req.Host, err)
		con.Close()
		return

	}
	c := s.clients.get(clientName)
	if c == nil {
		con.Close()
		return
	}
	id := s.nextID.Add(1)
	ch := s.pendings.add(id)
	message := protocol.Message{Type: protocol.TypeProxy, ID: id}
	if err := c.Send(message); err != nil {
		s.pendings.remove(id)
		con.Close()
		return
	}
	var data net.Conn
	select {
	case data = <-ch:
	case <-time.After(10 * time.Second):
		if _, ok := s.pendings.remove(id); ok {
			con.Close()
			return
		}
		data = <-ch
	}
	userConn := &tunnel.PrefixConn{Conn: con, R: io.MultiReader(&buf, con)}
	tunnel.Pipe(userConn, data)

}

func (s *server) handleRegisterLoop(con net.Conn, clientName string) {
	defer con.Close()

	c := &internal.Client{Con: con}
	if !s.clients.register(clientName, c) {
		log.Println("a client is already registered, rejecting", con.RemoteAddr())
		return
	}
	defer s.clients.remove(clientName, c) // runs before Close (defers are LIFO)

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
		s.handleRegisterLoop(con, m.Name)
	case protocol.TypeProxyOK:
		s.handleProxyOk(con, m.ID)
	default:
		con.Close()
		return
	}
}

func main() {
	s := &server{
		clients:  &registry{clients: make(map[string]*internal.Client)},
		pendings: &pendingMap{m: make(map[uint64]chan net.Conn)},
	}

	// Open both listeners before starting any goroutines, so a bad port
	// fails the program immediately.
	hostListener, err := net.Listen("tcp", HostAddr)
	if err != nil {
		log.Fatal("host listener: ", err)
	}
	controlListener, err := net.Listen("tcp", ControlAddr)
	if err != nil {
		log.Fatal("control listener: ", err)
	}
	log.Printf("listening: host %s, control %s", HostAddr, ControlAddr)

	// Buffered so the second loop can still send (and exit) after main stops reading.
	errc := make(chan error, 2)
	go func() { errc <- serve("host", hostListener, s.handleHostConnection) }()
	go func() { errc <- serve("control", controlListener, s.handleControlConnection) }()

	// Whichever accept loop stops first takes the whole server down.
	log.Fatal(<-errc)
}
