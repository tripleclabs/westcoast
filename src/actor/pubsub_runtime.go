package actor

import (
	"context"
	"fmt"
	"time"
)

const defaultBrokerMailboxCapacity = 65536

func (r *Runtime) emitBroker(brokerID string, op BrokerOperation, topicPattern string, subscriber PID, matched int, result BrokerOutcomeType, reason string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:      eid,
		Type:         EventBrokerLifecycle,
		ActorID:      brokerID,
		PIDNamespace: subscriber.Namespace,
		PIDActorID:   subscriber.ActorID,
		Timestamp:    r.now(),
		Result:       string(result),
		ErrorCode:    reason,
	})
	r.metrics.ObserveGuardrailDecision("pubsub_"+string(op), string(result))
	r.metrics.ObservePubSubOutcome(string(op), string(result), matched)
	r.outcomes.putBroker(BrokerOutcome{
		BrokerID:     brokerID,
		Operation:    op,
		TopicPattern: topicPattern,
		Subscriber:   subscriber,
		MatchedCount: matched,
		Result:       result,
		ReasonCode:   reason,
		At:           r.now(),
	})
}

func (r *Runtime) EnsureBrokerActor(brokerID string) (*ActorRef, error) {
	if brokerID == "" {
		brokerID = DefaultBrokerActorID
	}
	r.brokerMu.RLock()
	existing := r.brokers[brokerID]
	r.brokerMu.RUnlock()
	if existing != nil {
		return &ActorRef{runtime: r, actorID: brokerID}, nil
	}

	svc := newPubSubBrokerService(r, brokerID)
	ref, err := r.CreateActor(brokerID, nil, svc.handler, WithMailboxCapacity(defaultBrokerMailboxCapacity))
	if err != nil {
		if err == ErrDuplicateActorID {
			r.brokerMu.RLock()
			existing = r.brokers[brokerID]
			r.brokerMu.RUnlock()
			if existing != nil {
				return &ActorRef{runtime: r, actorID: brokerID}, nil
			}
		}
		return nil, err
	}
	r.brokerMu.Lock()
	r.brokers[brokerID] = svc
	r.brokerMu.Unlock()
	return ref, nil
}

func (r *Runtime) BrokerOutcomes(brokerID string) []BrokerOutcome {
	return r.outcomes.brokerByID(brokerID)
}

func (r *Runtime) BrokerPublishedCount(brokerID string) int {
	return r.outcomes.brokerPublishedCount(brokerID)
}

func (r *Runtime) BrokerSubscribe(ctx context.Context, brokerID string, subscriber PID, pattern string, timeout time.Duration) (BrokerCommandAck, error) {
	brokerID = defaultBrokerID(brokerID)
	broker, err := r.EnsureBrokerActor(brokerID)
	if err != nil {
		return BrokerCommandAck{}, err
	}
	res, err := broker.Ask(ctx, BrokerSubscribeCommand{Subscriber: subscriber, Pattern: pattern}, timeout)
	if err != nil {
		return BrokerCommandAck{}, err
	}
	ack, ok := res.Payload.(BrokerCommandAck)
	if !ok {
		return BrokerCommandAck{}, fmt.Errorf("%w: ack type", ErrPubSubInvalidCommand)
	}
	return ack, nil
}

func (r *Runtime) BrokerUnsubscribe(ctx context.Context, brokerID string, subscriber PID, pattern string, timeout time.Duration) (BrokerCommandAck, error) {
	brokerID = defaultBrokerID(brokerID)
	broker, err := r.EnsureBrokerActor(brokerID)
	if err != nil {
		return BrokerCommandAck{}, err
	}
	res, err := broker.Ask(ctx, BrokerUnsubscribeCommand{Subscriber: subscriber, Pattern: pattern}, timeout)
	if err != nil {
		return BrokerCommandAck{}, err
	}
	ack, ok := res.Payload.(BrokerCommandAck)
	if !ok {
		return BrokerCommandAck{}, fmt.Errorf("%w: ack type", ErrPubSubInvalidCommand)
	}
	return ack, nil
}

func (r *Runtime) BrokerPublish(ctx context.Context, brokerID, topic string, payload any, publisherActorID string) SubmitAck {
	brokerID = defaultBrokerID(brokerID)
	broker, err := r.EnsureBrokerActor(brokerID)
	if err != nil {
		return SubmitAck{Result: SubmitRejectedFound}
	}
	return broker.Send(ctx, BrokerPublishCommand{
		Topic:            topic,
		Payload:          payload,
		PublisherActorID: publisherActorID,
	})
}

func defaultBrokerID(brokerID string) string {
	if brokerID != "" {
		return brokerID
	}
	return DefaultBrokerActorID
}
