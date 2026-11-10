package tunnel

import (
	"io"
	"net"
	"sync"
)

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	}
}

// PrefixConn replays already-consumed bytes (via R) before reading from the
// underlying conn. Writes and Close go straight to the embedded conn.
type PrefixConn struct {
	net.Conn
	R io.Reader
}

func (p *PrefixConn) Read(b []byte) (int, error) { return p.R.Read(b) }

// CloseWrite forwards half-close to the underlying conn so Pipe still works.
func (p *PrefixConn) CloseWrite() error {
	if cw, ok := p.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return nil
}

func Pipe(a, b net.Conn) {
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(b, a)
		closeWrite(b)
	}()
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		closeWrite(a)
	}()
	wg.Wait()
}
