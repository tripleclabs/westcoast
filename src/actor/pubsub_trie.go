package actor

type trieNode struct {
	children map[string]*trieNode
	star     *trieNode
	exact    map[string]PID
	tail     map[string]PID
}

type subscriptionTrie struct {
	root *trieNode
}

func newSubscriptionTrie() *subscriptionTrie {
	return &subscriptionTrie{root: newTrieNode()}
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[string]*trieNode),
		exact:    make(map[string]PID),
		tail:     make(map[string]PID),
	}
}

func (t *subscriptionTrie) insert(p topicPattern, pid PID) {
	node := t.root
	for i, seg := range p.segments {
		if seg == "#" {
			node.tail[pid.Key()] = pid
			return
		}
		if seg == "+" {
			if node.star == nil {
				node.star = newTrieNode()
			}
			node = node.star
			continue
		}
		next := node.children[seg]
		if next == nil {
			next = newTrieNode()
			node.children[seg] = next
		}
		node = next
		if i == len(p.segments)-1 {
			node.exact[pid.Key()] = pid
		}
	}
}

func (t *subscriptionTrie) match(topic []string) []PID {
	results := make(map[string]PID)
	var walk func(node *trieNode, idx int)
	walk = func(node *trieNode, idx int) {
		if node == nil {
			return
		}
		for k, v := range node.tail {
			results[k] = v
		}
		if idx == len(topic) {
			for k, v := range node.exact {
				results[k] = v
			}
			return
		}
		seg := topic[idx]
		walk(node.children[seg], idx+1)
		walk(node.star, idx+1)
	}
	walk(t.root, 0)
	out := make([]PID, 0, len(results))
	for _, pid := range results {
		out = append(out, pid)
	}
	return out
}
