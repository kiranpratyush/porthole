package tunnel

import (
	"io"
	"net"
	"sync"
)

func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		tc.CloseWrite()
	}
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
