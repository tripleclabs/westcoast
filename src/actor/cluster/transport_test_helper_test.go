package cluster

import (
	"crypto/tls"
	"net"
	"os"
	"testing"
)

// newTestTransport creates a transport for tests using the registered
// default transport factory. Import grpctransport/default.go's init()
// via the link below to register it.
func newTestTransport(nodeID NodeID) Transport {
	if defaultTransportFactory == nil {
		panic("no default transport factory — grpctransport not linked")
	}
	return defaultTransportFactory(nodeID, nil)
}

func newTestTransportWithTLS(nodeID NodeID, tlsCfg *tls.Config) Transport {
	if defaultTransportFactory == nil {
		panic("no default transport factory — grpctransport not linked")
	}
	return defaultTransportFactory(nodeID, tlsCfg)
}

// testTransportAddr returns the listen address string for a transport.
func testTransportAddr(t Transport) string {
	type addrGetter interface {
		Addr() net.Addr
	}
	if ag, ok := t.(addrGetter); ok {
		if addr := ag.Addr(); addr != nil {
			return addr.String()
		}
	}
	return ""
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
