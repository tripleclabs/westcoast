package grpctransport

import (
	"crypto/tls"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

func init() {
	cluster.SetDefaultTransportFactory(func(nodeID cluster.NodeID, tlsCfg *tls.Config) cluster.Transport {
		if tlsCfg != nil {
			return NewWithConfig(nodeID, Config{
				ServerTLS: tlsCfg,
				ClientTLS: tlsCfg,
			})
		}
		return New(nodeID)
	})
}
