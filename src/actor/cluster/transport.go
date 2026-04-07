package cluster

import (
	"context"
	"crypto/tls"
	"errors"
)

var (
	ErrTransportClosed  = errors.New("transport_closed")
	ErrConnectionClosed = errors.New("connection_closed")
	ErrNodeUnreachable  = errors.New("node_unreachable")
	ErrDialFailed       = errors.New("dial_failed")
	ErrHandshakeFailed  = errors.New("handshake_failed")
	ErrSendFailed       = errors.New("send_failed")
	ErrEnvelopeTooLarge = errors.New("envelope_too_large")
)

// Transport abstracts the network layer for inter-node communication.
// Implementations handle connection management, multiplexing, and TLS.
type Transport interface {
	// Listen starts accepting inbound connections on the given address.
	// The handler receives callbacks for connection and envelope events.
	Listen(addr string, handler InboundHandler) error

	// Dial establishes an outbound connection to the given address.
	// Auth handshake is performed as part of the dial.
	Dial(ctx context.Context, addr string, auth ClusterAuth) (Connection, error)

	// Close shuts down the transport, closing all connections.
	Close() error
}

// Connection represents a bidirectional link to a remote node.
// A single connection multiplexes many actor-to-actor conversations.
type Connection interface {
	// Send transmits an envelope to the remote node.
	Send(ctx context.Context, env Envelope) error

	// Close terminates the connection.
	Close() error

	// RemoteAddr returns the address of the remote end.
	RemoteAddr() string

	// RemoteNodeID returns the node ID of the remote end,
	// established during handshake.
	RemoteNodeID() NodeID
}

// TransportFactory creates a Transport for a given node ID and optional
// TLS config. Registered via SetDefaultTransportFactory by transport
// packages (e.g. grpctransport).
type TransportFactory func(nodeID NodeID, tlsCfg *tls.Config) Transport

var defaultTransportFactory TransportFactory

// SetDefaultTransportFactory registers the factory used by cluster.Start()
// when no Transport is provided in the config. Import a transport package
// to set this automatically:
//
//	import _ 
func SetDefaultTransportFactory(f TransportFactory) {
	defaultTransportFactory = f
}

// InboundHandler processes events from incoming connections.
type InboundHandler interface {
	// OnEnvelope is called when a message arrives from a remote node.
	OnEnvelope(from NodeID, env Envelope)

	// OnConnectionEstablished is called when a new inbound connection completes handshake.
	OnConnectionEstablished(remote NodeID, conn Connection)

	// OnConnectionLost is called when an established connection drops.
	OnConnectionLost(remote NodeID, err error)
}
