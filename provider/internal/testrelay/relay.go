// Package testrelay supports fixture-owned plaintext database transport tests.
// It is never imported by a production provider.
package testrelay

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Stage struct {
	Bytes        int
	Disconnected <-chan struct{}
}

type Relay struct {
	Address     string
	Stage       <-chan Stage
	listener    net.Listener
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	done        chan struct{}
	workers     sync.WaitGroup
}

// Start gates one response only after the known bounded source SELECT has
// passed upstream and at least 64KiB has passed downstream. Traffic is neither
// logged nor persisted. TLS is deliberately outside this test-only profile.
func Start(target, source string) (*Relay, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	stages := make(chan Stage, 1)
	r := &Relay{Address: l.Addr().String(), Stage: stages, listener: l, connections: map[net.Conn]struct{}{}, done: make(chan struct{})}
	var gated atomic.Bool
	go func() {
		defer close(r.done)
		for {
			client, err := l.Accept()
			if err != nil {
				return
			}
			server, err := net.DialTimeout("tcp", target, 5*time.Second)
			if err != nil {
				_ = client.Close()
				continue
			}
			r.mu.Lock()
			r.connections[client] = struct{}{}
			r.connections[server] = struct{}{}
			r.mu.Unlock()
			r.workers.Add(1)
			go func() {
				defer r.workers.Done()
				defer func() {
					_ = client.Close()
					_ = server.Close()
					r.mu.Lock()
					delete(r.connections, client)
					delete(r.connections, server)
					r.mu.Unlock()
				}()
				var armed atomic.Bool
				disconnected := make(chan struct{})
				go func() {
					defer close(disconnected)
					defer func() { _ = server.Close() }()
					buffer := make([]byte, 32*1024)
					window := make([]byte, 0, 128*1024)
					for {
						n, e := client.Read(buffer)
						if n > 0 {
							if !armed.Load() && !gated.Load() {
								window = append(window, buffer[:n]...)
								if len(window) > 128*1024 {
									window = append(window[:0], window[len(window)-128*1024:]...)
								}
								if bytes.Contains(window, []byte(source)) && bytes.Contains(window, []byte("SELECT ")) && bytes.Contains(window, []byte(" ORDER BY ")) && bytes.Contains(window, []byte(" LIMIT 256")) {
									armed.Store(true)
									clear(window)
									window = nil
								}
							}
							if _, e := io.Copy(server, bytes.NewReader(buffer[:n])); e != nil {
								return
							}
						}
						if e != nil {
							return
						}
					}
				}()
				buffer := make([]byte, 32*1024)
				sent := 0
				for {
					n, e := server.Read(buffer)
					if n > 0 {
						if _, e := io.Copy(client, bytes.NewReader(buffer[:n])); e != nil {
							break
						}
						if armed.Load() {
							sent += n
							if sent >= 64*1024 && gated.CompareAndSwap(false, true) {
								stages <- Stage{sent, disconnected}
								<-disconnected
								break
							}
						}
					}
					if e != nil {
						break
					}
				}
				_ = client.Close()
				_ = server.Close()
				<-disconnected
			}()
		}
	}()
	return r, nil
}

func (r *Relay) Close() error {
	err := r.listener.Close()
	<-r.done
	r.mu.Lock()
	for c := range r.connections {
		_ = c.Close()
	}
	r.mu.Unlock()
	r.workers.Wait()
	return err
}
