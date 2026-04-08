package actor

import (
	"context"
	"fmt"
	"time"
)

const defaultCronMailboxCapacity = 65536

// EnsureCronActor creates the cron timer actor if it does not already exist.
// The actor is created lazily on first use.
func (r *Runtime) EnsureCronActor(cronID string) (*ActorRef, error) {
	cronID = defaultCronID(cronID)

	r.cronMu.RLock()
	existing := r.cronTimers[cronID]
	r.cronMu.RUnlock()
	if existing != nil {
		return &ActorRef{runtime: r, actorID: cronID}, nil
	}

	svc := newCronBrokerService(r, cronID)
	ref, err := r.CreateActor(cronID, nil, svc.handler, WithMailboxCapacity(defaultCronMailboxCapacity))
	if err != nil {
		if err == ErrDuplicateActorID {
			r.cronMu.RLock()
			existing = r.cronTimers[cronID]
			r.cronMu.RUnlock()
			if existing != nil {
				return &ActorRef{runtime: r, actorID: cronID}, nil
			}
		}
		return nil, err
	}
	r.cronMu.Lock()
	r.cronTimers[cronID] = svc
	r.cronMu.Unlock()
	return ref, nil
}

// CronHandler returns a handler for a cron timer actor without creating
// the actor. The service is registered internally so EnsureCronActor
// recognizes it. Used by daemon and singleton managers.
func (r *Runtime) CronHandler(cronID string) Handler {
	cronID = defaultCronID(cronID)

	r.cronMu.Lock()
	defer r.cronMu.Unlock()

	if svc, ok := r.cronTimers[cronID]; ok {
		return svc.handler
	}

	svc := newCronBrokerService(r, cronID)
	r.cronTimers[cronID] = svc
	return svc.handler
}

// CronSubscribe registers a cron-scheduled callback. Lazy-creates the
// cron timer actor if it does not already exist.
func (r *Runtime) CronSubscribe(ctx context.Context, cronID string, subscriber PID, schedule string, payload any, tag string, timeout time.Duration) (CronCommandAck, error) {
	cronID = defaultCronID(cronID)
	cron, err := r.EnsureCronActor(cronID)
	if err != nil {
		return CronCommandAck{}, err
	}
	res, err := cron.Ask(ctx, CronSubscribeCommand{
		Subscriber: subscriber,
		Schedule:   schedule,
		Payload:    payload,
		Tag:        tag,
	}, timeout)
	if err != nil {
		return CronCommandAck{}, err
	}
	ack, ok := res.Payload.(CronCommandAck)
	if !ok {
		return CronCommandAck{}, fmt.Errorf("%w: ack type", ErrCronInvalidCommand)
	}
	return ack, nil
}

// CronUnsubscribe cancels a cron subscription by ref ID.
func (r *Runtime) CronUnsubscribe(ctx context.Context, cronID string, subscriber PID, refID string, timeout time.Duration) (CronCommandAck, error) {
	cronID = defaultCronID(cronID)
	cron, err := r.EnsureCronActor(cronID)
	if err != nil {
		return CronCommandAck{}, err
	}
	res, err := cron.Ask(ctx, CronUnsubscribeCommand{
		Subscriber: subscriber,
		RefID:      refID,
	}, timeout)
	if err != nil {
		return CronCommandAck{}, err
	}
	ack, ok := res.Payload.(CronCommandAck)
	if !ok {
		return CronCommandAck{}, fmt.Errorf("%w: ack type", ErrCronInvalidCommand)
	}
	return ack, nil
}

// CronUnsubscribeAll cancels all cron subscriptions for a subscriber.
func (r *Runtime) CronUnsubscribeAll(ctx context.Context, cronID string, subscriber PID, timeout time.Duration) (CronCommandAck, error) {
	cronID = defaultCronID(cronID)
	cron, err := r.EnsureCronActor(cronID)
	if err != nil {
		return CronCommandAck{}, err
	}
	res, err := cron.Ask(ctx, CronUnsubscribeAllCommand{
		Subscriber: subscriber,
	}, timeout)
	if err != nil {
		return CronCommandAck{}, err
	}
	ack, ok := res.Payload.(CronCommandAck)
	if !ok {
		return CronCommandAck{}, fmt.Errorf("%w: ack type", ErrCronInvalidCommand)
	}
	return ack, nil
}

// CronList lists active cron subscriptions for a subscriber.
func (r *Runtime) CronList(ctx context.Context, cronID string, subscriber PID, timeout time.Duration) (CronCommandAck, error) {
	cronID = defaultCronID(cronID)
	cron, err := r.EnsureCronActor(cronID)
	if err != nil {
		return CronCommandAck{}, err
	}
	res, err := cron.Ask(ctx, CronListCommand{
		Subscriber: subscriber,
	}, timeout)
	if err != nil {
		return CronCommandAck{}, err
	}
	ack, ok := res.Payload.(CronCommandAck)
	if !ok {
		return CronCommandAck{}, fmt.Errorf("%w: ack type", ErrCronInvalidCommand)
	}
	return ack, nil
}

func defaultCronID(cronID string) string {
	if cronID != "" {
		return cronID
	}
	return DefaultCronActorID
}
