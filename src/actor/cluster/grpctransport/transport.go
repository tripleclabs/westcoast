package grpctransport

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Metadata keys for authentication.
const (
	metaNodeID    = "x-cluster-node"
	metaTimestamp = "x-cluster-ts"
	metaNonce     = "x-cluster-nonce"
	metaSignature = "x-cluster-sig"
)

// authTimestampSkew is the maximum allowed clock skew for HMAC auth.
const authTimestampSkew = 30 * time.Second

// Config provides optional configuration for the gRPC transport.
type Config struct {
	// ServerTLS configures TLS for the gRPC server (inbound connections).
	// For mTLS, set ClientAuth to tls.RequireAndVerifyClientCert and
	// provide a ClientCAs pool.
	// When nil, the server accepts plaintext connections.
	ServerTLS *tls.Config

	// ClientTLS configures TLS for outbound gRPC connections.
	// For mTLS, set Certificates with the client cert/key pair.
	// When nil, the client connects in plaintext.
	ClientTLS *tls.Config

	// ServerOptions are appended to the gRPC server options.
	ServerOptions []grpc.ServerOption

	// DialOptions are appended to the gRPC dial options.
	DialOptions []grpc.DialOption

	// DefaultHeaders are included on every outbound envelope.
	// Per-envelope headers (set via SendWithHeaders) take precedence.
	DefaultHeaders map[string]string
}

// GRPCTransport implements cluster.Transport over gRPC bidirectional streaming.
// Envelope routing metadata is native protobuf; the payload field carries
// gob-encoded actor messages so the existing codec pipeline is preserved.
//
// Authentication is handled at stream open via gRPC metadata:
//   - Shared secret: HMAC-SHA256 over nonce+timestamp, verified by a
//     stream interceptor. Replay-resistant via timestamp window + nonce.
//   - mTLS: configure ServerTLS with RequireAndVerifyClientCert and
//     ClientTLS with client certificates. The TLS layer handles mutual
//     authentication before any gRPC traffic flows.
//
// Both mechanisms can be used together (belt and suspenders).
type GRPCTransport struct {
	mu      sync.RWMutex
	localID cluster.NodeID
	auth    cluster.ClusterAuth
	handler cluster.InboundHandler
	cfg     Config

	server   *grpc.Server
	listener net.Listener
	closed   bool
}

// New creates a GRPCTransport with default (plaintext, no auth) settings.
func New(localID cluster.NodeID) *GRPCTransport {
	return &GRPCTransport{localID: localID}
}

// NewWithConfig creates a GRPCTransport with the given configuration.
func NewWithConfig(localID cluster.NodeID, cfg Config) *GRPCTransport {
	return &GRPCTransport{localID: localID, cfg: cfg}
}

// SetAuth sets the auth handler for verifying inbound connections.
// When set, all inbound streams must present valid HMAC credentials
// in gRPC metadata. Without auth, streams are accepted unconditionally.
func (t *GRPCTransport) SetAuth(auth cluster.ClusterAuth) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.auth = auth
}

// ---------------------------------------------------------------------------
// cluster.Transport implementation
// ---------------------------------------------------------------------------

func (t *GRPCTransport) Listen(addr string, handler cluster.InboundHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return cluster.ErrTransportClosed
	}
	t.handler = handler

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%w: %v", cluster.ErrDialFailed, err)
	}
	t.listener = lis

	var opts []grpc.ServerOption
	if t.cfg.ServerTLS != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(t.cfg.ServerTLS)))
	}
	opts = append(opts, grpc.StreamInterceptor(t.streamAuthInterceptor))
	opts = append(opts, t.cfg.ServerOptions...)

	t.server = grpc.NewServer(opts...)
	RegisterClusterTransportServer(t.server, &serverImpl{transport: t})

	go t.server.Serve(lis)
	return nil
}

func (t *GRPCTransport) Dial(ctx context.Context, addr string, auth cluster.ClusterAuth) (cluster.Connection, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, cluster.ErrTransportClosed
	}
	t.mu.RUnlock()

	var opts []grpc.DialOption
	if t.cfg.ClientTLS != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(t.cfg.ClientTLS)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	opts = append(opts, t.cfg.DialOptions...)

	cc, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", cluster.ErrDialFailed, err)
	}

	// The stream must outlive the caller's context (which may have a
	// short timeout for dialling). We use a detached context for the
	// stream and cancel it when the connection is closed.
	streamCtx, streamCancel := context.WithCancel(context.Background())

	// Build auth metadata.
	if auth != nil {
		md, err := t.buildAuthMetadata(auth)
		if err != nil {
			streamCancel()
			cc.Close()
			return nil, fmt.Errorf("%w: %v", cluster.ErrHandshakeFailed, err)
		}
		streamCtx = metadata.NewOutgoingContext(streamCtx, md)
	}

	client := NewClusterTransportClient(cc)
	stream, err := client.Stream(streamCtx)
	if err != nil {
		streamCancel()
		cc.Close()
		return nil, fmt.Errorf("%w: %v", cluster.ErrHandshakeFailed, err)
	}

	// Read the server's node ID from response headers.
	headerMD, err := stream.Header()
	if err != nil {
		streamCancel()
		cc.Close()
		return nil, fmt.Errorf("%w: %v", cluster.ErrHandshakeFailed, err)
	}
	remoteNode := cluster.NodeID(firstMDValue(headerMD, metaNodeID))

	conn := &grpcConn{
		cc:           cc,
		stream:       stream,
		streamCancel: streamCancel,
		remoteNode:   remoteNode,
		remoteAddr:   addr,
		headers:      t.cfg.DefaultHeaders,
	}

	// Start a read loop for incoming messages from the server on this
	// bidirectional stream. This enables the server to send responses
	// (e.g. singleton discover responses, CRDT deltas) back through
	// the same stream the client opened.
	t.mu.RLock()
	handler := t.handler
	t.mu.RUnlock()
	if handler != nil {
		go func() {
			for {
				pb, err := stream.Recv()
				if err != nil {
					return
				}
				handler.OnEnvelope(remoteNode, protoToEnvelope(pb))
			}
		}()
	}

	return conn, nil
}

func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.server != nil {
		t.server.Stop()
	}
	if t.listener != nil {
		t.listener.Close()
	}
	return nil
}

// Addr returns the listener address, useful in tests with ":0" ports.
func (t *GRPCTransport) Addr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}

// ---------------------------------------------------------------------------
// HMAC metadata auth
// ---------------------------------------------------------------------------

// buildAuthMetadata produces the gRPC metadata for HMAC-based auth.
// The HMAC key is derived from auth.Credentials().
func (t *GRPCTransport) buildAuthMetadata(auth cluster.ClusterAuth) (metadata.MD, error) {
	secret, err := auth.Credentials()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	nonceHex := hex.EncodeToString(nonce)
	sig := computeHMAC(secret, nonceHex, ts)

	return metadata.Pairs(
		metaNodeID, string(t.localID),
		metaTimestamp, ts,
		metaNonce, nonceHex,
		metaSignature, sig,
	), nil
}

// streamAuthInterceptor is a gRPC server stream interceptor that verifies
// HMAC credentials in metadata before allowing the stream to proceed.
func (t *GRPCTransport) streamAuthInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	t.mu.RLock()
	auth := t.auth
	t.mu.RUnlock()

	if auth == nil {
		return handler(srv, ss)
	}

	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	ts := firstMDValue(md, metaTimestamp)
	nonce := firstMDValue(md, metaNonce)
	sig := firstMDValue(md, metaSignature)

	if ts == "" || nonce == "" || sig == "" {
		return status.Error(codes.Unauthenticated, "missing auth metadata")
	}

	// Verify timestamp is within acceptable skew.
	tsNano, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid timestamp")
	}
	delta := time.Since(time.Unix(0, tsNano))
	if delta < 0 {
		delta = -delta
	}
	if delta > authTimestampSkew {
		return status.Error(codes.Unauthenticated, "timestamp outside acceptable window")
	}

	// Recompute HMAC using server's secret.
	secret, err := auth.Credentials()
	if err != nil {
		return status.Error(codes.Internal, "auth credentials unavailable")
	}

	expected := computeHMAC(secret, nonce, ts)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return status.Error(codes.Unauthenticated, "invalid signature")
	}

	return handler(srv, ss)
}

func computeHMAC(secret []byte, nonce, timestamp string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(nonce))
	mac.Write([]byte("|"))
	mac.Write([]byte(timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

func firstMDValue(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// ---------------------------------------------------------------------------
// gRPC server implementation
// ---------------------------------------------------------------------------

type serverImpl struct {
	UnimplementedClusterTransportServer
	transport *GRPCTransport
}

func (s *serverImpl) Stream(stream ClusterTransport_StreamServer) error {
	// Send our node ID as response header metadata so the dialling
	// side can learn who it connected to.
	s.transport.mu.RLock()
	localID := s.transport.localID
	s.transport.mu.RUnlock()

	if err := stream.SendHeader(metadata.Pairs(metaNodeID, string(localID))); err != nil {
		return err
	}

	// The first envelope identifies the remote node.
	first, err := stream.Recv()
	if err != nil {
		return err
	}

	remoteID := cluster.NodeID(first.SenderNode)

	// Extract peer address from gRPC transport.
	var remoteAddr string
	if p, ok := peer.FromContext(stream.Context()); ok {
		remoteAddr = p.Addr.String()
	}

	conn := &grpcServerConn{
		stream:     stream,
		remoteNode: remoteID,
		remoteAddr: remoteAddr,
	}

	s.transport.mu.RLock()
	handler := s.transport.handler
	s.transport.mu.RUnlock()

	if handler != nil {
		handler.OnConnectionEstablished(remoteID, conn)
	}

	// Deliver the first envelope.
	if handler != nil {
		handler.OnEnvelope(remoteID, protoToEnvelope(first))
	}

	// Read loop.
	for {
		pb, err := stream.Recv()
		if err != nil {
			if handler != nil {
				handler.OnConnectionLost(remoteID, err)
			}
			return nil
		}
		if handler != nil {
			handler.OnEnvelope(remoteID, protoToEnvelope(pb))
		}
	}
}

// ---------------------------------------------------------------------------
// Outbound connection (client-side)
// ---------------------------------------------------------------------------

type grpcConn struct {
	mu           sync.Mutex
	cc           *grpc.ClientConn
	stream       ClusterTransport_StreamClient
	streamCancel context.CancelFunc
	remoteNode   cluster.NodeID
	remoteAddr   string
	headers      map[string]string
	closed       bool
}

func (c *grpcConn) Send(ctx context.Context, env cluster.Envelope) error {
	pb := envelopeToProto(env, c.headers)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return cluster.ErrConnectionClosed
	}
	return c.stream.Send(pb)
}

func (c *grpcConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.streamCancel()
	c.stream.CloseSend()
	return c.cc.Close()
}

func (c *grpcConn) RemoteAddr() string          { return c.remoteAddr }
func (c *grpcConn) RemoteNodeID() cluster.NodeID { return c.remoteNode }

// ---------------------------------------------------------------------------
// Inbound connection (server-side, wraps a stream)
// ---------------------------------------------------------------------------

type grpcServerConn struct {
	stream     ClusterTransport_StreamServer
	remoteNode cluster.NodeID
	remoteAddr string
}

func (c *grpcServerConn) Send(ctx context.Context, env cluster.Envelope) error {
	return c.stream.Send(envelopeToProto(env, nil))
}

func (c *grpcServerConn) Close() error {
	// Server-side streams are closed when the handler returns.
	return nil
}

func (c *grpcServerConn) RemoteAddr() string          { return c.remoteAddr }
func (c *grpcServerConn) RemoteNodeID() cluster.NodeID { return c.remoteNode }

// ---------------------------------------------------------------------------
// Envelope conversion
// ---------------------------------------------------------------------------

func envelopeToProto(env cluster.Envelope, headers map[string]string) *ClusterEnvelope {
	pb := &ClusterEnvelope{
		SenderNode:     string(env.SenderNode),
		SenderActorId:  env.SenderActorID,
		TargetNode:     string(env.TargetNode),
		TargetActorId:  env.TargetActorID,
		Namespace:      env.Namespace,
		Generation:     env.Generation,
		TypeName:       env.TypeName,
		SchemaVersion:  env.SchemaVersion,
		MessageId:      env.MessageID,
		Payload:        env.Payload,
		IsAsk:          env.IsAsk,
		AskRequestId:   env.AskRequestID,
		SentAtUnixNano: env.SentAtUnixNano,
	}
	if env.AskReplyTo != nil {
		pb.AskReplyTo = &RemotePIDProto{
			Node:       string(env.AskReplyTo.Node),
			Namespace:  env.AskReplyTo.Namespace,
			ActorId:    env.AskReplyTo.ActorID,
			Generation: env.AskReplyTo.Generation,
		}
	}
	if len(headers) > 0 {
		pb.Headers = make(map[string]string, len(headers))
		for k, v := range headers {
			pb.Headers[k] = v
		}
	}
	return pb
}

func protoToEnvelope(pb *ClusterEnvelope) cluster.Envelope {
	env := cluster.Envelope{
		SenderNode:     cluster.NodeID(pb.SenderNode),
		SenderActorID:  pb.SenderActorId,
		TargetNode:     cluster.NodeID(pb.TargetNode),
		TargetActorID:  pb.TargetActorId,
		Namespace:      pb.Namespace,
		Generation:     pb.Generation,
		TypeName:       pb.TypeName,
		SchemaVersion:  pb.SchemaVersion,
		MessageID:      pb.MessageId,
		Payload:        pb.Payload,
		IsAsk:          pb.IsAsk,
		AskRequestID:   pb.AskRequestId,
		SentAtUnixNano: pb.SentAtUnixNano,
	}
	if pb.AskReplyTo != nil {
		env.AskReplyTo = &cluster.RemotePID{
			Node:       cluster.NodeID(pb.AskReplyTo.Node),
			Namespace:  pb.AskReplyTo.Namespace,
			ActorID:    pb.AskReplyTo.ActorId,
			Generation: pb.AskReplyTo.Generation,
		}
	}
	return env
}

// ---------------------------------------------------------------------------
// Header helpers
// ---------------------------------------------------------------------------

// HeaderKey constants for common gateway/routing headers.
const (
	HeaderTraceID    = "x-trace-id"
	HeaderTenantID   = "x-tenant-id"
	HeaderRoutingKey = "x-routing-key"
	HeaderPriority   = "x-priority"
	HeaderTTL        = "x-ttl"
)

// HeaderConn extends cluster.Connection with per-message header support.
// Connections returned by GRPCTransport.Dial implement this interface.
type HeaderConn interface {
	SendWithHeaders(ctx context.Context, env cluster.Envelope, headers map[string]string) error
}

// SendWithHeaders sends an envelope with additional per-message headers
// merged on top of the transport's DefaultHeaders.
func (c *grpcConn) SendWithHeaders(ctx context.Context, env cluster.Envelope, headers map[string]string) error {
	merged := make(map[string]string, len(c.headers)+len(headers))
	for k, v := range c.headers {
		merged[k] = v
	}
	for k, v := range headers {
		merged[k] = v
	}
	pb := envelopeToProto(env, merged)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return cluster.ErrConnectionClosed
	}
	return c.stream.Send(pb)
}

// Verify interface compliance at compile time.
var (
	_ cluster.Transport  = (*GRPCTransport)(nil)
	_ cluster.Connection = (*grpcConn)(nil)
	_ cluster.Connection = (*grpcServerConn)(nil)
	_ HeaderConn         = (*grpcConn)(nil)
)

