# nggrok Roadmap

nggrok is a learning-focused ngrok clone written in Go. A client behind NAT dials *out* to a public server, and the server routes public traffic back down that tunnel to a local app.

Each phase ends with something runnable from two terminals. Use only the standard library until Phase 6. Don't touch a VPS until Phase 7.

## Layout

```
cmd/server   # public server (package main)
cmd/client   # tunnel client (package main)
internal/    # shared code: protocol, framing, helpers
```

---

## ✅ Phase 1: TCP basics
- [x] Echo server and client
- [x] Local port forwarder (`:8000` → `:3000`), `io.Copy` in both directions
- [x] Handle half-close correctly (`CloseWrite()`)

**Learned:** `net.Listen`, `net.Dial`, goroutines, `io.Copy`, clean connection shutdown

## ✅ Phase 2: Naive reverse tunnel
- [x] Server with host listener (`:8080`) and control listener (`:8081`)
- [x] Client dials out; server pipes one public connection down the single client connection
- [x] Python smoke test passing

**Learned:** connection lifetimes, blocking I/O, why multiplexing is needed

## ✅ Phase 3: Control protocol + dial-back
- [x] Length-prefixed frame protocol (`Register`, `RegisterOK`, `NewConn{id}`, `PROXY_OK`)
- [x] Separate control and data connections (control, user, data, app)
- [x] Pending map pairing user conns with dial-back data conns
- [x] Mutex-guarded client registry
- [x] Dial-back timeout with pending map as the single authority on who "wins"
- [x] Client: register with retry, control read loop, `handleProxyConnection`
- [x] Server: `handleUser` piping with timeout and ownership rules

**Learned:** `encoding/binary`, `encoding/json`, `bufio`, `sync.Mutex`, channels for pairing, `select`

---

## ⬜ Phase 4: HTTP + subdomain routing
- [ ] Parse the `Host` header and extract the subdomain
- [ ] Route each request to the matching client in the registry
- [ ] Clients register a requested subdomain; reject duplicates
- [ ] Test locally: `curl -H "Host: abc.localhost" localhost:8080`
- [ ] Optional: `httputil.ReverseProxy` with a custom `DialContext` that opens a stream through the tunnel

**Learn:** `net/http`, `httputil`, interfaces

## ⬜ Phase 5: Robustness
- [ ] Heartbeats on the control connection
- [ ] Client reconnect with exponential backoff
- [ ] Remove a client from the registry when it dies
- [ ] Timeouts everywhere (read, write, dial-back)
- [ ] Auth token on register
- [ ] Graceful shutdown for server and client
- [ ] Run tests with `-race`; parallel load test (e.g. 20 concurrent sessions)

**Learn:** `context`, `time.Ticker`, `select`, `errgroup`

## ⬜ Phase 6: Your own stream multiplexer
- [ ] Replace dial-back with many logical streams over one TCP connection
- [ ] Frame format: `streamID | flags | length | payload`
- [ ] Stream open/close (SYN/FIN-style flags)
- [ ] Per-stream buffers
- [ ] Basic flow control (windowing)
- [ ] Compare against `hashicorp/yamux`

**Learn:** framing, per-stream state, flow control basics (a mini HTTP/2 framing layer)

## ⬜ Phase 7: Go live
- [ ] TLS on the control channel
- [ ] Deploy the server to a cheap VPS
- [ ] Wildcard DNS: `*.nggrok.<yourdomain>`
- [ ] Wildcard certificate via Let's Encrypt (DNS-01 challenge)
- [ ] Cross-compile client binaries

**Learn:** `crypto/tls`, `autocert` / `certmagic`, cross-compiling

## ⬜ Phase 8: Extras (pick what interests you)
- [ ] Request inspector UI (like ngrok's `localhost:4040`)
- [ ] Raw TCP tunnels (one public port per tunnel)
- [ ] Metrics

---

## Tips
- Watch your frames with Wireshark or `tcpdump` on loopback.
- Write a short design doc before coding each phase (connections, lifecycles, failure cases), as done for Phase 3.
