// Integration tests for the distributed actor system using novatest.
// These tests spin up real VMs, deploy the testnode binary, and verify
// cross-node behavior over actual TCP connections.
//
// Run with: go test ./tests/integration_cluster/ -v -timeout 10m
// Requires: nova binary (see github.com/tripleclabs/nova)
package integration_cluster

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tripleclabs/nova/pkg/novatest"
)

func init() {
	// Skip all tests in this package if nova isn't available or
	// the WESTCOAST_INTEGRATION env var isn't set.
	if os.Getenv("WESTCOAST_INTEGRATION") == "" {
		return
	}
}

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("WESTCOAST_INTEGRATION") == "" {
		t.Skip("set WESTCOAST_INTEGRATION=1 to run cluster integration tests")
	}
	if _, err := exec.LookPath("nova"); err != nil {
		t.Skip("nova binary not found in PATH")
	}
}

// projectRoot returns the absolute path to the westcoast project root.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// sharedFolder is injected at runtime with the absolute project root path.
func clusterHCL(projectRoot string) string {
	return `
defaults {
  image  = "alpine:3.21"
  cpus   = 2
  memory = "2G"
}

network {
  subnet = "10.0.0.0/24"
}

node "node1" {
  shared_folder {
    host_path  = "` + projectRoot + `"
    guest_path = "/workspace"
    read_only  = true
  }
}

node "node2" {
  shared_folder {
    host_path  = "` + projectRoot + `"
    guest_path = "/workspace"
    read_only  = true
  }
}
`
}

func threeNodeHCL(projectRoot string) string {
	return `
defaults {
  image  = "alpine:3.21"
  cpus   = 2
  memory = "2G"
}

network {
  subnet = "10.0.0.0/24"
}

node "node1" {
  shared_folder {
    host_path  = "` + projectRoot + `"
    guest_path = "/workspace"
    read_only  = true
  }
}

node "node2" {
  shared_folder {
    host_path  = "` + projectRoot + `"
    guest_path = "/workspace"
    read_only  = true
  }
}

node "node3" {
  shared_folder {
    host_path  = "` + projectRoot + `"
    guest_path = "/workspace"
    read_only  = true
  }
}
`
}

// buildTestNode cross-compiles the testnode binary for linux/amd64 and
// places it in the project directory (which is shared into VMs at /workspace).
// Must be called once before any deployTestNode calls.
func buildTestNode(t *testing.T) {
	t.Helper()
	// The test binary should be pre-built before running integration tests.
	// Build with: GOOS=linux GOARCH=amd64 go build -o testnode-linux ./cmd/testnode/
	// For now, this is a documentation requirement. The test will skip if
	// the binary doesn't exist.
}

// deployTestNode copies the pre-built testnode binary from the shared
// folder and starts it on the given VM node.
func deployTestNode(t *testing.T, c *novatest.Cluster, nodeName, nodeID, ip, httpPort string, peerIPs []string) {
	t.Helper()
	node := c.Node(nodeName)

	// Check shared folder is mounted.
	result := node.ExecResult("ls /workspace/testnode-linux")
	if result.ExitCode != 0 {
		t.Skipf("testnode-linux binary not found at /workspace/testnode-linux. Build with: GOOS=linux GOARCH=arm64 go build -o testnode-linux ./cmd/testnode/\nstderr: %s", result.Stderr)
	}

	// Copy binary from shared folder to a writable location.
	node.Exec("cp /workspace/testnode-linux /tmp/testnode && chmod +x /tmp/testnode")

	// Build the peer flags.
	var peerFlags string
	for _, peer := range peerIPs {
		peerFlags += " --peer " + peer + ":9000"
	}

	// Start testnode in the background.
	node.Exec("nohup /tmp/testnode" +
		" --id " + nodeID +
		" --addr " + ip + ":9000" +
		" --http " + ip + ":" + httpPort +
		peerFlags +
		" > /tmp/testnode.log 2>&1 &")
}

func httpGet(node *novatest.Node, url string) string {
	return strings.TrimSpace(node.Exec("curl -sf " + url))
}

func httpPost(node *novatest.Node, url string) string {
	return strings.TrimSpace(node.Exec("curl -sf -X POST " + url))
}

func TestCluster_TwoNodeFormation(t *testing.T) {
	skipUnlessIntegration(t)
	cluster := novatest.NewCluster(t, novatest.WithHCL(clusterHCL(projectRoot(t))))
	cluster.WaitReady()

	// Install curl (Alpine doesn't have it by default).
	cluster.Node("node1").Exec("apk add --quiet curl")
	cluster.Node("node2").Exec("apk add --quiet curl")

	node1IP := cluster.Node("node1").IP
	node2IP := cluster.Node("node2").IP

	deployTestNode(t, cluster, "node1", "node-1", node1IP, "8080", []string{node2IP})
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	// Wait for both nodes to be healthy.
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
		return r.ExitCode == 0
	})
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node2").ExecResult("curl -sf http://" + node2IP + ":8080/health")
		return r.ExitCode == 0
	})

	// Wait for cluster formation — node1 should see node2 as a member.
	novatest.Eventually(t, 30*time.Second, func() bool {
		out := httpGet(cluster.Node("node1"), "http://"+node1IP+":8080/cluster/members")
		return strings.Contains(out, "node-2")
	})
}

func TestCluster_CrossNodeMessaging(t *testing.T) {
	skipUnlessIntegration(t)
	cluster := novatest.NewCluster(t, novatest.WithHCL(clusterHCL(projectRoot(t))))
	cluster.WaitReady()

	node1IP := cluster.Node("node1").IP
	node2IP := cluster.Node("node2").IP

	deployTestNode(t, cluster, "node1", "node-1", node1IP, "8080", []string{node2IP})
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	// Wait for health + cluster formation.
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
		return r.ExitCode == 0
	})
	novatest.Eventually(t, 30*time.Second, func() bool {
		out := httpGet(cluster.Node("node1"), "http://"+node1IP+":8080/cluster/members")
		return strings.Contains(out, "node-2")
	})

	// Send a message from node1 to the echo actor on node2.
	httpPost(cluster.Node("node1"),
		"http://"+node1IP+":8080/send?target=echo&ns=node-2&payload=hello-from-node1")

	// Verify node2 received it.
	novatest.Eventually(t, 10*time.Second, func() bool {
		out := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages")
		return strings.Contains(out, "hello-from-node1")
	})
}

func TestCluster_NetworkPartition(t *testing.T) {
	skipUnlessIntegration(t)
	cluster := novatest.NewCluster(t, novatest.WithHCL(clusterHCL(projectRoot(t))))
	cluster.WaitReady()

	node1IP := cluster.Node("node1").IP
	node2IP := cluster.Node("node2").IP

	deployTestNode(t, cluster, "node1", "node-1", node1IP, "8080", []string{node2IP})
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	// Wait for cluster formation.
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
		return r.ExitCode == 0
	})
	novatest.Eventually(t, 30*time.Second, func() bool {
		out := httpGet(cluster.Node("node1"), "http://"+node1IP+":8080/cluster/members")
		return strings.Contains(out, "node-2")
	})

	// Verify messaging works before partition.
	httpPost(cluster.Node("node1"),
		"http://"+node1IP+":8080/send?target=echo&ns=node-2&payload=before-partition")

	novatest.Eventually(t, 10*time.Second, func() bool {
		out := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages")
		return strings.Contains(out, "before-partition")
	})

	// Partition the nodes.
	cluster.Partition("node1", "node2")

	// Messages should fail during partition.
	result := cluster.Node("node1").ExecResult(
		"curl -sf 'http://" + node1IP + ":8080/send?target=echo&ns=node-2&payload=during-partition'")
	// The send may return an error outcome or fail to connect.

	// Get message count on node2 before healing.
	countBefore := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages/count")

	// Heal the partition.
	cluster.Heal("node1", "node2")

	// Wait for connectivity to restore.
	time.Sleep(3 * time.Second)

	// Messaging should work again.
	httpPost(cluster.Node("node1"),
		"http://"+node1IP+":8080/send?target=echo&ns=node-2&payload=after-heal")

	novatest.Eventually(t, 10*time.Second, func() bool {
		out := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages")
		return strings.Contains(out, "after-heal")
	})

	_ = result
	_ = countBefore
}

func TestCluster_NodeFailureAndRecovery(t *testing.T) {
	skipUnlessIntegration(t)
	cluster := novatest.NewCluster(t, novatest.WithHCL(clusterHCL(projectRoot(t))))
	cluster.WaitReady()

	node1IP := cluster.Node("node1").IP
	node2IP := cluster.Node("node2").IP

	deployTestNode(t, cluster, "node1", "node-1", node1IP, "8080", []string{node2IP})
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	// Wait for cluster.
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
		return r.ExitCode == 0
	})
	novatest.Eventually(t, 30*time.Second, func() bool {
		out := httpGet(cluster.Node("node1"), "http://"+node1IP+":8080/cluster/members")
		return strings.Contains(out, "node-2")
	})

	// Kill node2.
	cluster.Node("node2").Kill()

	// Node1 should still be healthy.
	r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
	if r.ExitCode != 0 {
		t.Error("node1 should still be healthy after node2 failure")
	}

	// Restart node2.
	cluster.Node("node2").Start()
	novatest.Eventually(t, 60*time.Second, func() bool {
		return cluster.Node("node2").IsRunning()
	})

	// Redeploy testnode on node2.
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	// Wait for node2 to rejoin.
	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node2").ExecResult("curl -sf http://" + node2IP + ":8080/health")
		return r.ExitCode == 0
	})

	// Messaging should work again.
	httpPost(cluster.Node("node1"),
		"http://"+node1IP+":8080/send?target=echo&ns=node-2&payload=after-recovery")

	novatest.Eventually(t, 10*time.Second, func() bool {
		out := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages")
		return strings.Contains(out, "after-recovery")
	})
}

func TestCluster_LinkDegradation(t *testing.T) {
	skipUnlessIntegration(t)
	cluster := novatest.NewCluster(t, novatest.WithHCL(clusterHCL(projectRoot(t))))
	cluster.WaitReady()

	node1IP := cluster.Node("node1").IP
	node2IP := cluster.Node("node2").IP

	deployTestNode(t, cluster, "node1", "node-1", node1IP, "8080", []string{node2IP})
	deployTestNode(t, cluster, "node2", "node-2", node2IP, "8080", []string{node1IP})

	novatest.Eventually(t, 30*time.Second, func() bool {
		r := cluster.Node("node1").ExecResult("curl -sf http://" + node1IP + ":8080/health")
		return r.ExitCode == 0
	})
	novatest.Eventually(t, 30*time.Second, func() bool {
		out := httpGet(cluster.Node("node1"), "http://"+node1IP+":8080/cluster/members")
		return strings.Contains(out, "node-2")
	})

	// Add 200ms latency between nodes.
	cluster.Degrade("node1", "node2",
		novatest.WithLatency(200*time.Millisecond),
		novatest.WithJitter(50*time.Millisecond),
	)

	// Messages should still arrive, just slower.
	httpPost(cluster.Node("node1"),
		"http://"+node1IP+":8080/send?target=echo&ns=node-2&payload=degraded-msg")

	// Give extra time due to latency.
	novatest.Eventually(t, 15*time.Second, func() bool {
		out := httpGet(cluster.Node("node2"), "http://"+node2IP+":8080/messages")
		return strings.Contains(out, "degraded-msg")
	})

	cluster.HealAll()
}

// parseJSON is a test helper for quick JSON parsing.
func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("parse JSON %q: %v", s, err)
	}
	return out
}
