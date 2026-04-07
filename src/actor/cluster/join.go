package cluster

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

// Join protocol errors.
var (
	ErrJoinBadToken  = errors.New("join: token verification failed")
	ErrJoinRejected  = errors.New("join: request rejected by server")
	ErrJoinBadMagic  = errors.New("join: invalid magic bytes")
)

const (
	joinMagic    = "JREQ"
	nonceSize    = 32
	joinTimeout  = 10 // seconds, for individual read/write ops
)

// JoinResponse contains the data returned by a successful join handshake.
type JoinResponse struct {
	NodeCertDER []byte // signed node certificate (DER)
	CACertDER   []byte // cluster CA certificate (DER)
	CAKeyPKCS8  []byte // cluster CA private key (PKCS8)
	ServerID    string // node ID of the server that accepted the join
	ClusterAddr string // server's cluster transport address (for FixedProvider seeds)
	Payload     []byte // optional app-level payload (nil if none)
}

// JoinRequest contains the data sent by a joining node.
type JoinRequest struct {
	NodeID      string // joining node's ID
	ClusterAddr string // joining node's cluster transport address
	CSRDER      []byte // certificate signing request (DER)
}

// JoinServer accepts join requests from new nodes. It runs on every node
// that has the cluster CA key.
type JoinServer struct {
	token    []byte
	listener net.Listener
	handler  func(req JoinRequest) (*JoinResponse, error)

	mu     sync.Mutex
	closed bool
}

// NewJoinServer creates a join server that listens on addr. The handler
// is called for each authenticated join request and must return the
// signed cert, CA cert, CA key, and optional payload.
func NewJoinServer(addr string, token string, handler func(JoinRequest) (*JoinResponse, error)) (*JoinServer, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("join server: listen %s: %w", addr, err)
	}

	js := &JoinServer{
		token:    []byte(token),
		listener: lis,
		handler:  handler,
	}
	return js, nil
}

// Addr returns the listener address.
func (js *JoinServer) Addr() string { return js.listener.Addr().String() }

// Serve accepts connections until the listener is closed.
func (js *JoinServer) Serve() {
	for {
		conn, err := js.listener.Accept()
		if err != nil {
			js.mu.Lock()
			closed := js.closed
			js.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go js.handleConn(conn)
	}
}

// Close shuts down the join server.
func (js *JoinServer) Close() error {
	js.mu.Lock()
	js.closed = true
	js.mu.Unlock()
	return js.listener.Close()
}

func (js *JoinServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// Step 1: Read JREQ + client nonce.
	magic := make([]byte, 4)
	if _, err := io.ReadFull(conn, magic); err != nil {
		return
	}
	if string(magic) != joinMagic {
		return
	}

	nonceC := make([]byte, nonceSize)
	if _, err := io.ReadFull(conn, nonceC); err != nil {
		return
	}

	// Step 2: Send server nonce + HMAC(token, nonceC || nonceS).
	nonceS := make([]byte, nonceSize)
	if _, err := rand.Read(nonceS); err != nil {
		return
	}

	serverHMAC := computeJoinHMAC(js.token, nonceC, nonceS)

	if _, err := conn.Write(nonceS); err != nil {
		return
	}
	if _, err := conn.Write(serverHMAC); err != nil {
		return
	}

	// Step 3: Read client HMAC + node ID + CSR.
	clientHMAC := make([]byte, sha256.Size)
	if _, err := io.ReadFull(conn, clientHMAC); err != nil {
		return
	}

	expectedClientHMAC := computeJoinHMAC(js.token, nonceS, nonceC)
	if !hmac.Equal(clientHMAC, expectedClientHMAC) {
		return // wrong token
	}

	nodeID, err := readLenPrefixed(conn)
	if err != nil {
		return
	}
	clientClusterAddr, err := readLenPrefixed(conn)
	if err != nil {
		return
	}
	csrDER, err := readLenPrefixed(conn)
	if err != nil {
		return
	}

	// Call handler to sign the CSR and get response data.
	resp, err := js.handler(JoinRequest{
		NodeID:      string(nodeID),
		ClusterAddr: string(clientClusterAddr),
		CSRDER:      csrDER,
	})
	if err != nil {
		return
	}

	// Step 4: Send signed cert + CA cert + CA key + server ID + cluster addr + payload.
	writeLenPrefixed(conn, resp.NodeCertDER)
	writeLenPrefixed(conn, resp.CACertDER)
	writeLenPrefixed(conn, resp.CAKeyPKCS8)
	writeLenPrefixed(conn, []byte(resp.ServerID))
	writeLenPrefixed(conn, []byte(resp.ClusterAddr))
	if resp.Payload != nil {
		writeLenPrefixed(conn, resp.Payload)
	} else {
		writeLenPrefixed(conn, []byte{})
	}
}

// JoinClient performs a join handshake with a server at addr.
func JoinClient(ctx context.Context, addr string, token string, req JoinRequest) (*JoinResponse, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("join client: dial %s: %w", addr, err)
	}
	defer conn.Close()

	tokenBytes := []byte(token)

	// Step 1: Send JREQ + client nonce.
	nonceC := make([]byte, nonceSize)
	if _, err := rand.Read(nonceC); err != nil {
		return nil, fmt.Errorf("join client: generate nonce: %w", err)
	}

	if _, err := conn.Write([]byte(joinMagic)); err != nil {
		return nil, fmt.Errorf("join client: write magic: %w", err)
	}
	if _, err := conn.Write(nonceC); err != nil {
		return nil, fmt.Errorf("join client: write nonce: %w", err)
	}

	// Step 2: Read server nonce + server HMAC.
	nonceS := make([]byte, nonceSize)
	if _, err := io.ReadFull(conn, nonceS); err != nil {
		return nil, fmt.Errorf("join client: read server nonce: %w", err)
	}

	serverHMAC := make([]byte, sha256.Size)
	if _, err := io.ReadFull(conn, serverHMAC); err != nil {
		return nil, fmt.Errorf("join client: read server HMAC: %w", err)
	}

	// Verify server's HMAC — proves server knows the token.
	expectedServerHMAC := computeJoinHMAC(tokenBytes, nonceC, nonceS)
	if !hmac.Equal(serverHMAC, expectedServerHMAC) {
		return nil, ErrJoinBadToken
	}

	// Step 3: Send client HMAC + node ID + CSR.
	clientHMAC := computeJoinHMAC(tokenBytes, nonceS, nonceC)
	if _, err := conn.Write(clientHMAC); err != nil {
		return nil, fmt.Errorf("join client: write HMAC: %w", err)
	}
	if err := writeLenPrefixed(conn, []byte(req.NodeID)); err != nil {
		return nil, fmt.Errorf("join client: write node ID: %w", err)
	}
	if err := writeLenPrefixed(conn, []byte(req.ClusterAddr)); err != nil {
		return nil, fmt.Errorf("join client: write cluster addr: %w", err)
	}
	if err := writeLenPrefixed(conn, req.CSRDER); err != nil {
		return nil, fmt.Errorf("join client: write CSR: %w", err)
	}

	// Step 4: Read response.
	certDER, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read cert: %w", err)
	}
	caCertDER, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read CA cert: %w", err)
	}
	caKeyPKCS8, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read CA key: %w", err)
	}
	serverID, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read server ID: %w", err)
	}
	clusterAddr, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read cluster addr: %w", err)
	}
	payload, err := readLenPrefixed(conn)
	if err != nil {
		return nil, fmt.Errorf("join client: read payload: %w", err)
	}

	var payloadOut []byte
	if len(payload) > 0 {
		payloadOut = payload
	}

	return &JoinResponse{
		NodeCertDER: certDER,
		CACertDER:   caCertDER,
		CAKeyPKCS8:  caKeyPKCS8,
		ServerID:    string(serverID),
		ClusterAddr: string(clusterAddr),
		Payload:     payloadOut,
	}, nil
}

// --- helpers ---

func computeJoinHMAC(key, a, b []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(a)
	h.Write(b)
	return h.Sum(nil)
}

func writeLenPrefixed(w io.Writer, data []byte) error {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func readLenPrefixed(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > 1<<20 { // 1MB sanity limit
		return nil, fmt.Errorf("join: message too large: %d bytes", n)
	}
	data := make([]byte, n)
	if n > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
	}
	return data, nil
}
