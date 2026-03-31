// testnode is a small binary for integration testing the distributed actor system.
// It starts a clustered Runtime with HTTP introspection endpoints that test
// harnesses (like novatest) can drive.
//
// Usage:
//
//	testnode --id node-1 --addr 10.0.0.2:9000 --http :8080 --peer 10.0.0.3:9000 --peer 10.0.0.4:9000
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

type peers []string

func (p *peers) String() string { return strings.Join(*p, ",") }
func (p *peers) Set(v string) error {
	*p = append(*p, v)
	return nil
}

func main() {
	var (
		nodeID   string
		addr     string
		httpAddr string
		peerList peers
	)
	flag.StringVar(&nodeID, "id", "", "node ID (required)")
	flag.StringVar(&addr, "addr", ":9000", "cluster transport listen address")
	flag.StringVar(&httpAddr, "http", ":8080", "HTTP introspection listen address")
	flag.Var(&peerList, "peer", "peer in id@addr format, e.g. node-2@10.0.0.3:9000 (repeatable)")
	flag.Parse()

	if nodeID == "" {
		log.Fatal("--id is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build cluster infrastructure.
	codec := cluster.NewGobCodec()
	registerTestTypes(codec)

	// Parse peers from id@addr format into NodeMeta seeds.
	var seeds []cluster.NodeMeta
	for _, p := range peerList {
		parts := strings.SplitN(p, "@", 2)
		if len(parts) != 2 {
			log.Fatalf("invalid peer format %q, expected id@addr", p)
		}
		seeds = append(seeds, cluster.NodeMeta{ID: cluster.NodeID(parts[0]), Addr: parts[1]})
	}

	transport := cluster.NewGRPCTransport(cluster.NodeID(nodeID))
	provider := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		Seeds:             seeds,
		HeartbeatInterval: 2 * time.Second,
		FailureThreshold:  3,
	})

	// Probe peers by attempting a TCP dial.
	provider.Probe = func(ctx context.Context, addr string) error {
		conn, err := transport.Dial(ctx, addr, cluster.NoopAuth{})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	c, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: cluster.NodeID(nodeID), Addr: addr},
		Provider:  provider,
		Transport: transport,
		Auth:      cluster.NoopAuth{},
		Codec:     codec,
		Topology:  cluster.FullMeshTopology{},
	})
	if err != nil {
		log.Fatalf("cluster: %v", err)
	}

	// Build runtime with cluster integration.
	remoteSender := cluster.NewRemoteSender(c, codec, nil)
	remoteResolver := cluster.NewRemotePIDResolver(actor.NewInMemoryPIDResolver(), cluster.NodeID(nodeID))

	rt := actor.NewRuntime(
		actor.WithNodeID(nodeID),
		actor.WithRemoteSend(remoteSender.Send),
	)

	// Inbound dispatcher.
	dispatcher := cluster.NewInboundDispatcher(rt, codec)
	dispatcher.SetCluster(c)
	c.SetOnEnvelope(func(from cluster.NodeID, env cluster.Envelope) {
		dispatcher.Dispatch(ctx, from, env)
	})

	// Track membership for the remote resolver.
	c.SetOnMemberEvent(func(ev cluster.MemberEvent) {
		switch ev.Type {
		case cluster.MemberJoin:
			remoteResolver.AddRemoteNode(ev.Member.ID)
		case cluster.MemberLeave, cluster.MemberFailed:
			remoteResolver.RemoveRemoteNode(ev.Member.ID)
		}
		rt.PublishMembershipEvent(ctx, actor.ClusterMembershipEvent{
			Type: ev.Type.String(),
			Member: actor.ClusterMemberInfo{
				ID:   string(ev.Member.ID),
				Addr: ev.Member.Addr,
			},
		})
	})

	// Start cluster.
	if err := c.Start(ctx); err != nil {
		log.Fatalf("cluster start: %v", err)
	}
	defer c.Stop()

	// Create the echo actor — receives messages and stores them.
	app := newTestApp(rt)
	app.setup(ctx)

	// Start HTTP introspection server.
	mux := http.NewServeMux()
	app.registerRoutes(mux, c, remoteResolver)

	server := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("[%s] HTTP listening on %s", nodeID, httpAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	log.Printf("[%s] cluster listening on %s, peers: %v", nodeID, addr, peerList)

	// Wait for interrupt.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("[%s] shutting down", nodeID)
	server.Shutdown(ctx)
}

// testApp holds the test actors and their state.
type testApp struct {
	rt *actor.Runtime

	mu       sync.Mutex
	received []receivedMsg
}

type receivedMsg struct {
	ActorID string `json:"actor_id"`
	Payload any    `json:"payload"`
	At      string `json:"at"`
}

func newTestApp(rt *actor.Runtime) *testApp {
	return &testApp{rt: rt}
}

func (a *testApp) setup(ctx context.Context) {
	// Create an "echo" actor that stores received messages.
	a.rt.CreateActor("echo", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		a.mu.Lock()
		a.received = append(a.received, receivedMsg{
			ActorID: msg.ActorID,
			Payload: msg.Payload,
			At:      time.Now().Format(time.RFC3339Nano),
		})
		a.mu.Unlock()

		// If it's an ask, reply with the payload echoed back.
		if replyTo, ok := msg.AskReplyTo(); ok {
			a.rt.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   msg.Payload,
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	})

	// Issue a PID for the echo actor.
	a.rt.IssuePID("", "echo")

	// Ensure the pubsub broker exists.
	a.rt.EnsureBrokerActor("")
}

func (a *testApp) registerRoutes(mux *http.ServeMux, c *cluster.Cluster, resolver *cluster.RemotePIDResolver) {
	// GET /health — liveness check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// GET /cluster/members — current membership
	mux.HandleFunc("/cluster/members", func(w http.ResponseWriter, r *http.Request) {
		members := c.Members()
		out := make([]map[string]string, len(members))
		for i, m := range members {
			out[i] = map[string]string{"id": string(m.ID), "addr": m.Addr}
		}
		json.NewEncoder(w).Encode(out)
	})

	// GET /cluster/connected — nodes with active connections
	mux.HandleFunc("/cluster/connected", func(w http.ResponseWriter, r *http.Request) {
		members := c.Members()
		var connected []string
		for _, m := range members {
			if c.IsConnected(m.ID) {
				connected = append(connected, string(m.ID))
			}
		}
		json.NewEncoder(w).Encode(connected)
	})

	// POST /send?target=<actorID>&ns=<namespace>&payload=<string>
	// Sends a message to an actor by PID.
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		ns := r.URL.Query().Get("ns")
		payload := r.URL.Query().Get("payload")

		if ns == "" {
			ns = a.rt.NodeID()
		}

		pid := actor.PID{Namespace: ns, ActorID: target, Generation: 1}
		ack := a.rt.SendPID(r.Context(), pid, payload)
		json.NewEncoder(w).Encode(map[string]string{
			"outcome":    string(ack.Outcome),
			"message_id": fmt.Sprintf("%d", ack.MessageID),
		})
	})

	// GET /messages — received messages on the echo actor
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		msgs := make([]receivedMsg, len(a.received))
		copy(msgs, a.received)
		a.mu.Unlock()
		json.NewEncoder(w).Encode(msgs)
	})

	// GET /messages/count — count of received messages
	mux.HandleFunc("/messages/count", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		n := len(a.received)
		a.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]int{"count": n})
	})

	// POST /ask?target=<actorID>&ns=<namespace>&payload=<string>&timeout=<duration>
	// Asks an actor by PID and returns the reply.
	mux.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		ns := r.URL.Query().Get("ns")
		payload := r.URL.Query().Get("payload")
		timeoutStr := r.URL.Query().Get("timeout")
		if timeoutStr == "" {
			timeoutStr = "5s"
		}
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			http.Error(w, "bad timeout: "+err.Error(), 400)
			return
		}
		if ns == "" {
			ns = a.rt.NodeID()
		}

		pid := actor.PID{Namespace: ns, ActorID: target, Generation: 1}
		result, err := a.rt.AskPID(r.Context(), pid, payload, timeout)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"payload": result.Payload, "request_id": result.RequestID})
	})

	// POST /publish?topic=<topic>&payload=<string>
	mux.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Query().Get("topic")
		payload := r.URL.Query().Get("payload")
		ack := a.rt.BrokerPublish(r.Context(), "", topic, payload, "http")
		json.NewEncoder(w).Encode(map[string]string{"result": string(ack.Result)})
	})

	// POST /subscribe?pattern=<pattern>
	// Subscribes the echo actor to a pubsub pattern.
	mux.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		pattern := r.URL.Query().Get("pattern")
		pid, ok := a.rt.PIDForActor("echo")
		if !ok {
			http.Error(w, "echo actor has no PID", 500)
			return
		}
		ack, err := a.rt.BrokerSubscribe(r.Context(), "", pid, pattern, 5*time.Second)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"result": string(ack.Result)})
	})

	// POST /register?name=<name>
	// Registers the echo actor with a name.
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		ack, err := a.rt.RegisterName("echo", name, a.rt.NodeID())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"result": string(ack.Result)})
	})

	// GET /lookup?name=<name>
	mux.HandleFunc("/lookup", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		ack := a.rt.LookupName(name)
		json.NewEncoder(w).Encode(map[string]string{
			"result":    string(ack.Result),
			"namespace": ack.PID.Namespace,
			"actor_id":  ack.PID.ActorID,
		})
	})
}

func registerTestTypes(codec *cluster.GobCodec) {
	codec.Register("")
	codec.Register(map[string]any{})
	codec.Register(actor.BrokerPublishedMessage{})
	codec.Register(actor.BrokerPublishCommand{})
	codec.Register(actor.BrokerSubscribeCommand{})
	codec.Register(actor.BrokerUnsubscribeCommand{})
	codec.Register(actor.BrokerCommandAck{})
	codec.Register(actor.AskReplyEnvelope{})
	codec.Register(actor.ClusterMembershipEvent{})
	codec.Register(actor.ClusterMemberInfo{})
}
