package actor

import (
	"context"
	"sync"
)

const DefaultBrokerActorID = "__native_pubsub_broker"

type pubsubBrokerService struct {
	runtime      *Runtime
	brokerID     string
	mu           sync.RWMutex
	trie         *subscriptionTrie
	byPattern    map[string]map[string]PID
	bySubscriber map[string]map[string]struct{}
}

func newPubSubBrokerService(runtime *Runtime, brokerID string) *pubsubBrokerService {
	return &pubsubBrokerService{
		runtime:      runtime,
		brokerID:     brokerID,
		trie:         newSubscriptionTrie(),
		byPattern:    make(map[string]map[string]PID),
		bySubscriber: make(map[string]map[string]struct{}),
	}
}

func (b *pubsubBrokerService) handler(ctx context.Context, state any, msg Message) (any, error) {
	switch cmd := msg.Payload.(type) {
	case BrokerSubscribeCommand:
		b.handleSubscribe(ctx, msg, cmd)
	case BrokerUnsubscribeCommand:
		b.handleUnsubscribe(ctx, msg, cmd)
	case BrokerPublishCommand:
		b.handlePublish(ctx, cmd)
	default:
		_ = isBrokerCommand(msg.Payload)
		b.runtime.emitBroker(b.brokerID, BrokerOperationPublish, "", PID{}, 0, BrokerOutcomeInvalidCommand, ErrPubSubInvalidCommand.Error())
		if replyTo, ok := msg.AskReplyTo(); ok {
			b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
				Operation:  BrokerOperationPublish,
				Result:     BrokerOutcomeInvalidCommand,
				ReasonCode: ErrPubSubInvalidCommand.Error(),
			})
		}
	}
	return state, nil
}

func (b *pubsubBrokerService) handleSubscribe(ctx context.Context, msg Message, cmd BrokerSubscribeCommand) {
	if !msg.IsAsk() {
		b.runtime.emitBroker(b.brokerID, BrokerOperationSubscribe, cmd.Pattern, cmd.Subscriber, 0, BrokerOutcomeInvalidCommand, ErrPubSubInvalidCommand.Error())
		return
	}
	p, err := parseTopicPattern(cmd.Pattern)
	if err != nil {
		b.runtime.emitBroker(b.brokerID, BrokerOperationSubscribe, cmd.Pattern, cmd.Subscriber, 0, BrokerOutcomeInvalidPattern, ErrPubSubInvalidPattern.Error())
		replyTo, _ := msg.AskReplyTo()
		b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
			Operation:    BrokerOperationSubscribe,
			Result:       BrokerOutcomeInvalidPattern,
			TopicPattern: cmd.Pattern,
			ReasonCode:   ErrPubSubInvalidPattern.Error(),
		})
		return
	}

	key := cmd.Subscriber.Key()
	b.mu.Lock()
	if b.byPattern[p.raw] == nil {
		b.byPattern[p.raw] = make(map[string]PID)
	}
	_, exists := b.byPattern[p.raw][key]
	b.byPattern[p.raw][key] = cmd.Subscriber
	if b.bySubscriber[key] == nil {
		b.bySubscriber[key] = make(map[string]struct{})
	}
	b.bySubscriber[key][p.raw] = struct{}{}
	if !exists {
		b.trie.insert(p, cmd.Subscriber)
	}
	b.mu.Unlock()

	b.runtime.emitBroker(b.brokerID, BrokerOperationSubscribe, p.raw, cmd.Subscriber, 0, BrokerOutcomeSubscribeSuccess, "")
	replyTo, _ := msg.AskReplyTo()
	b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
		Operation:    BrokerOperationSubscribe,
		Result:       BrokerOutcomeSubscribeSuccess,
		TopicPattern: p.raw,
	})
}

func (b *pubsubBrokerService) handleUnsubscribe(ctx context.Context, msg Message, cmd BrokerUnsubscribeCommand) {
	if !msg.IsAsk() {
		b.runtime.emitBroker(b.brokerID, BrokerOperationUnsubscribe, cmd.Pattern, cmd.Subscriber, 0, BrokerOutcomeInvalidCommand, ErrPubSubInvalidCommand.Error())
		return
	}
	p, err := parseTopicPattern(cmd.Pattern)
	if err != nil {
		b.runtime.emitBroker(b.brokerID, BrokerOperationUnsubscribe, cmd.Pattern, cmd.Subscriber, 0, BrokerOutcomeInvalidPattern, ErrPubSubInvalidPattern.Error())
		replyTo, _ := msg.AskReplyTo()
		b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
			Operation:    BrokerOperationUnsubscribe,
			Result:       BrokerOutcomeInvalidPattern,
			TopicPattern: cmd.Pattern,
			ReasonCode:   ErrPubSubInvalidPattern.Error(),
		})
		return
	}

	key := cmd.Subscriber.Key()
	b.mu.Lock()
	_, hasSub := b.byPattern[p.raw][key]
	if hasSub {
		delete(b.byPattern[p.raw], key)
		if len(b.byPattern[p.raw]) == 0 {
			delete(b.byPattern, p.raw)
		}
		delete(b.bySubscriber[key], p.raw)
		if len(b.bySubscriber[key]) == 0 {
			delete(b.bySubscriber, key)
		}
		b.rebuildTrieLocked()
	}
	b.mu.Unlock()

	replyTo, _ := msg.AskReplyTo()
	if !hasSub {
		b.runtime.emitBroker(b.brokerID, BrokerOperationUnsubscribe, p.raw, cmd.Subscriber, 0, BrokerOutcomeNotFoundNoop, "")
		b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
			Operation:    BrokerOperationUnsubscribe,
			Result:       BrokerOutcomeNotFoundNoop,
			TopicPattern: p.raw,
		})
		return
	}
	b.runtime.emitBroker(b.brokerID, BrokerOperationUnsubscribe, p.raw, cmd.Subscriber, 0, BrokerOutcomeUnsubscribeSuccess, "")
	b.replyAsk(ctx, msg.AskRequestID(), replyTo, BrokerCommandAck{
		Operation:    BrokerOperationUnsubscribe,
		Result:       BrokerOutcomeUnsubscribeSuccess,
		TopicPattern: p.raw,
	})
}

func (b *pubsubBrokerService) handlePublish(ctx context.Context, cmd BrokerPublishCommand) {
	topic, err := parsePublishTopic(cmd.Topic)
	if err != nil {
		b.runtime.emitBroker(b.brokerID, BrokerOperationPublish, cmd.Topic, PID{}, 0, BrokerOutcomeInvalidTopic, ErrPubSubInvalidTopic.Error())
		return
	}
	b.mu.RLock()
	targets := b.trie.match(topic)
	b.mu.RUnlock()
	b.dispatchPublish(ctx, cmd, targets)
}

func (b *pubsubBrokerService) dispatchPublish(ctx context.Context, cmd BrokerPublishCommand, targets []PID) {
	if len(targets) == 0 {
		b.runtime.emitBroker(b.brokerID, BrokerOperationPublish, cmd.Topic, PID{}, 0, BrokerOutcomePublishSuccess, "")
		return
	}
	go func() {
		unreachable := 0
		for _, pid := range targets {
			ack := b.runtime.SendPID(ctx, pid, BrokerPublishedMessage{
				Topic:            cmd.Topic,
				Payload:          cmd.Payload,
				PublisherActorID: cmd.PublisherActorID,
				PublishedAt:      b.runtime.now(),
			})
			if ack.Outcome != PIDDelivered {
				unreachable++
			}
		}
		result, reason := classifyPublishResult(unreachable, len(targets))
		b.runtime.emitBroker(b.brokerID, BrokerOperationPublish, cmd.Topic, PID{}, len(targets), result, reason)
	}()
}

func (b *pubsubBrokerService) rebuildTrieLocked() {
	b.trie = newSubscriptionTrie()
	for pattern, subscribers := range b.byPattern {
		p, err := parseTopicPattern(pattern)
		if err != nil {
			continue
		}
		for _, pid := range subscribers {
			b.trie.insert(p, pid)
		}
	}
}

func (b *pubsubBrokerService) replyAsk(ctx context.Context, requestID string, replyTo PID, payload BrokerCommandAck) {
	_ = b.runtime.SendPID(ctx, replyTo, AskReplyEnvelope{
		RequestID: requestID,
		Payload:   payload,
		RepliedAt: b.runtime.now(),
	})
}
