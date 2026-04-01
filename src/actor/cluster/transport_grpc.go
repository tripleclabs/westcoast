package cluster

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"sync"
)

func init() {
	gob.Register(Envelope{})
	gob.Register(RemotePID{})
}

// GRPCTransport implements Transport using persistent TCP connections with
// length-prefixed gob-encoded frames. Despite the name (retained for API
// compatibility), this is a raw TCP transport — simpler and more predictable
// than gRPC streaming for actor message passing.
//
// Wire protocol:
//
//	[4 bytes big-endian length][gob-encoded Envelope]
//
// Handshake protocol (on connection establishment):
//
//	client → server: [4 bytes len]["HELO"][4 bytes len][node-id][4 bytes len][auth-credentials]
//	server → client: [4 bytes len]["OK"] or connection closed on rejection
type GRPCTransport struct {
	mu      sync.RWMutex
	handler InboundHandler
	localID NodeID
	auth    ClusterAuth

	listener net.Listener
	closed   bool
}

func NewGRPCTransport(localID NodeID) *GRPCTransport {
	return &GRPCTransport{localID: localID}
}

func (t *GRPCTransport) Listen(addr string, handler InboundHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}
	t.handler = handler

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDialFailed, err)
	}
	t.listener = lis

	go t.acceptLoop(lis)
	return nil
}

func (t *GRPCTransport) acceptLoop(lis net.Listener) {
	for {
		raw, err := lis.Accept()
		if err != nil {
			return // listener closed
		}
		go t.handleInbound(raw)
	}
}

func (t *GRPCTransport) handleInbound(raw net.Conn) {
	// Handshake: read HELO, node ID, auth credentials.
	magic, err := readFrame(raw)
	if err != nil || string(magic) != "HELO" {
		raw.Close()
		return
	}

	nodeIDBytes, err := readFrame(raw)
	if err != nil {
		raw.Close()
		return
	}
	remoteID := NodeID(nodeIDBytes)

	authBytes, err := readFrame(raw)
	if err != nil {
		raw.Close()
		return
	}

	// Verify auth.
	t.mu.RLock()
	auth := t.auth
	t.mu.RUnlock()

	if auth != nil {
		if err := auth.Verify(authBytes); err != nil {
			raw.Close()
			return
		}
	}

	// Send OK.
	if err := writeFrame(raw, []byte("OK")); err != nil {
		raw.Close()
		return
	}

	conn := &tcpConn{
		raw:        raw,
		remoteNode: remoteID,
		remoteAddr: raw.RemoteAddr().String(),
	}

	t.mu.RLock()
	handler := t.handler
	t.mu.RUnlock()

	if handler != nil {
		handler.OnConnectionEstablished(remoteID, conn)
	}

	// Read loop: receive envelopes.
	for {
		frame, err := readFrame(raw)
		if err != nil {
			if handler != nil {
				handler.OnConnectionLost(remoteID, err)
			}
			return
		}

		env, err := decodeEnvelope(frame)
		if err != nil {
			continue
		}

		if handler != nil {
			handler.OnEnvelope(remoteID, env)
		}
	}
}

func (t *GRPCTransport) Dial(ctx context.Context, addr string, auth ClusterAuth) (Connection, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()

	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDialFailed, err)
	}

	// Handshake: send HELO, node ID, auth credentials.
	if err := writeFrame(raw, []byte("HELO")); err != nil {
		raw.Close()
		return nil, fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
	}
	if err := writeFrame(raw, []byte(t.localID)); err != nil {
		raw.Close()
		return nil, fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
	}

	var authBytes []byte
	if auth != nil {
		authBytes, err = auth.Credentials()
		if err != nil {
			raw.Close()
			return nil, fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
		}
	}
	if err := writeFrame(raw, authBytes); err != nil {
		raw.Close()
		return nil, fmt.Errorf("%w: %v", ErrHandshakeFailed, err)
	}

	// Wait for OK.
	resp, err := readFrame(raw)
	if err != nil || string(resp) != "OK" {
		raw.Close()
		return nil, fmt.Errorf("%w: rejected", ErrHandshakeFailed)
	}

	return &tcpConn{
		raw:        raw,
		remoteNode: NodeID(""), // set by caller if needed
		remoteAddr: addr,
	}, nil
}

func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.listener != nil {
		t.listener.Close()
	}
	return nil
}

// SetAuth sets the auth handler for verifying inbound connections.
func (t *GRPCTransport) SetAuth(auth ClusterAuth) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.auth = auth
}

// --- TCP connection ---

type tcpConn struct {
	raw        net.Conn
	remoteNode NodeID
	remoteAddr string
	writeMu    sync.Mutex
	closed     bool
}

func (c *tcpConn) Send(ctx context.Context, env Envelope) error {
	data, err := encodeEnvelope(env)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return ErrConnectionClosed
	}
	return writeFrame(c.raw, data)
}

func (c *tcpConn) Close() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.raw.Close()
}

func (c *tcpConn) RemoteAddr() string   { return c.remoteAddr }
func (c *tcpConn) RemoteNodeID() NodeID { return c.remoteNode }

// --- Wire encoding helpers ---

const maxFrameSize = 16 * 1024 * 1024 // 16 MB

func writeFrame(w io.Writer, data []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(hdr[:])
	if size > maxFrameSize {
		return nil, ErrEnvelopeTooLarge
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func encodeEnvelope(env Envelope) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(env); err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeEnvelope(data []byte) (Envelope, error) {
	var env Envelope
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return env, nil
}
