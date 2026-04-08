package actor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

const DefaultCronActorID = "__native_cron_timer"
const SingletonCronActorID = "__native_cron_singleton_timer"

// cronSubscription tracks a single cron registration.
type cronSubscription struct {
	ref        CronRef
	subscriber PID
	actorID    string // stable actor ID (survives restarts)
	schedule   cron.Schedule
	payload    any
	tag        string
	monitorRef MonitorRef
	cancel     context.CancelFunc
}

// cronBrokerService is the core cron timer actor. It manages cron
// subscriptions and fires CronTickMessages on schedule. Mirrors the
// pattern established by pubsubBrokerService.
type cronBrokerService struct {
	runtime *Runtime
	cronID  string

	mu       sync.Mutex
	parser   cron.Parser
	subs     map[string]*cronSubscription   // refID → subscription
	byActor  map[string]map[string]struct{} // actorID → set of refIDs
	seq      atomic.Uint64
	restored bool // true after handoff state has been restored
}

func newCronBrokerService(runtime *Runtime, cronID string) *cronBrokerService {
	return &cronBrokerService{
		runtime: runtime,
		cronID:  cronID,
		parser:  cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		subs:    make(map[string]*cronSubscription),
		byActor: make(map[string]map[string]struct{}),
	}
}

func (cs *cronBrokerService) handler(ctx context.Context, state any, msg Message) (any, error) {
	// On first message after singleton handoff, restore subscriptions.
	if !cs.restored {
		cs.restored = true
		if hs, ok := state.(*CronHandoffState); ok && hs != nil {
			cs.restoreFromHandoff(hs)
		}
	}

	switch cmd := msg.Payload.(type) {
	case CronSubscribeCommand:
		cs.handleSubscribe(ctx, msg, cmd)
	case CronUnsubscribeCommand:
		cs.handleUnsubscribe(ctx, msg, cmd)
	case CronUnsubscribeAllCommand:
		cs.handleUnsubscribeAll(ctx, msg, cmd)
	case CronListCommand:
		cs.handleList(ctx, msg, cmd)
	case DownMessage:
		cs.handleDown(cmd)
	default:
		if replyTo, ok := msg.AskReplyTo(); ok {
			cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
				Operation:  CronOperationSubscribe,
				Result:     CronOutcomeInvalidCommand,
				ReasonCode: ErrCronInvalidCommand.Error(),
			})
		}
	}
	return cs.captureState(), nil
}

func (cs *cronBrokerService) handleSubscribe(ctx context.Context, msg Message, cmd CronSubscribeCommand) {
	if !msg.IsAsk() {
		return
	}
	replyTo, _ := msg.AskReplyTo()

	sched, err := cs.parser.Parse(cmd.Schedule)
	if err != nil {
		cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
			Operation:  CronOperationSubscribe,
			Result:     CronOutcomeInvalidSchedule,
			ReasonCode: ErrCronInvalidSchedule.Error(),
		})
		return
	}

	refID := fmt.Sprintf("cron-%s-%d", cs.cronID, cs.seq.Add(1))
	ref := CronRef{
		ID:       refID,
		Schedule: cmd.Schedule,
		ActorID:  cmd.Subscriber.ActorID,
	}

	// Monitor the subscriber for terminal stop.
	cronPID := PID{Namespace: cmd.Subscriber.Namespace, ActorID: cs.cronID, Generation: 1}
	monRef := cs.runtime.Monitor(cronPID, cmd.Subscriber)

	subCtx, cancel := context.WithCancel(context.Background())
	sub := &cronSubscription{
		ref:        ref,
		subscriber: cmd.Subscriber,
		actorID:    cmd.Subscriber.ActorID,
		schedule:   sched,
		payload:    cmd.Payload,
		tag:        cmd.Tag,
		monitorRef: monRef,
		cancel:     cancel,
	}

	cs.mu.Lock()
	cs.subs[refID] = sub
	if cs.byActor[sub.actorID] == nil {
		cs.byActor[sub.actorID] = make(map[string]struct{})
	}
	cs.byActor[sub.actorID][refID] = struct{}{}
	cs.mu.Unlock()

	go cs.runSchedule(subCtx, sub)

	cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
		Operation: CronOperationSubscribe,
		Result:    CronOutcomeSubscribeSuccess,
		Ref:       ref,
	})
}

func (cs *cronBrokerService) handleUnsubscribe(ctx context.Context, msg Message, cmd CronUnsubscribeCommand) {
	if !msg.IsAsk() {
		return
	}
	replyTo, _ := msg.AskReplyTo()

	cs.mu.Lock()
	sub, ok := cs.subs[cmd.RefID]
	if ok {
		cs.removeLocked(cmd.RefID, sub)
	}
	cs.mu.Unlock()

	if !ok {
		cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
			Operation:  CronOperationUnsubscribe,
			Result:     CronOutcomeNotFound,
			ReasonCode: ErrCronRefNotFound.Error(),
		})
		return
	}

	cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
		Operation: CronOperationUnsubscribe,
		Result:    CronOutcomeUnsubscribeSuccess,
		Ref:       sub.ref,
	})
}

func (cs *cronBrokerService) handleUnsubscribeAll(ctx context.Context, msg Message, cmd CronUnsubscribeAllCommand) {
	if !msg.IsAsk() {
		return
	}
	replyTo, _ := msg.AskReplyTo()

	cs.mu.Lock()
	refIDs := cs.byActor[cmd.Subscriber.ActorID]
	for refID := range refIDs {
		if sub, ok := cs.subs[refID]; ok {
			cs.removeLocked(refID, sub)
		}
	}
	cs.mu.Unlock()

	cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
		Operation: CronOperationUnsubscribeAll,
		Result:    CronOutcomeUnsubscribeSuccess,
	})
}

func (cs *cronBrokerService) handleList(ctx context.Context, msg Message, cmd CronListCommand) {
	if !msg.IsAsk() {
		return
	}
	replyTo, _ := msg.AskReplyTo()

	cs.mu.Lock()
	refIDs := cs.byActor[cmd.Subscriber.ActorID]
	refs := make([]CronRef, 0, len(refIDs))
	for refID := range refIDs {
		if sub, ok := cs.subs[refID]; ok {
			refs = append(refs, sub.ref)
		}
	}
	cs.mu.Unlock()

	cs.replyAsk(ctx, msg.AskRequestID(), replyTo, CronCommandAck{
		Operation: CronOperationList,
		Result:    CronOutcomeListSuccess,
		Refs:      refs,
	})
}

func (cs *cronBrokerService) handleDown(down DownMessage) {
	actorID := down.Target.ActorID
	cs.mu.Lock()
	refIDs := cs.byActor[actorID]
	for refID := range refIDs {
		if sub, ok := cs.subs[refID]; ok {
			sub.cancel()
			delete(cs.subs, refID)
		}
	}
	delete(cs.byActor, actorID)
	cs.mu.Unlock()
}

// removeLocked cancels and removes a subscription. Must be called with cs.mu held.
func (cs *cronBrokerService) removeLocked(refID string, sub *cronSubscription) {
	sub.cancel()
	cs.runtime.Demonitor(sub.monitorRef)
	delete(cs.subs, refID)
	if refs, ok := cs.byActor[sub.actorID]; ok {
		delete(refs, refID)
		if len(refs) == 0 {
			delete(cs.byActor, sub.actorID)
		}
	}
}

// runSchedule is the per-subscription goroutine that fires CronTickMessages.
func (cs *cronBrokerService) runSchedule(ctx context.Context, sub *cronSubscription) {
	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		next := sub.schedule.Next(now)
		if next.IsZero() {
			return // schedule has no more fires
		}
		delay := next.Sub(now)
		select {
		case <-time.After(delay):
			if ctx.Err() != nil {
				return
			}
			// Send by actor ID to survive PID generation bumps on restart.
			cs.runtime.Send(context.Background(), sub.actorID, CronTickMessage{
				Ref:     sub.ref,
				Payload: sub.payload,
				FiredAt: next,
				Tag:     sub.tag,
			})
		case <-ctx.Done():
			return
		}
	}
}

// captureState serializes all subscriptions for singleton handoff.
func (cs *cronBrokerService) captureState() *CronHandoffState {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	entries := make([]CronHandoffEntry, 0, len(cs.subs))
	for _, sub := range cs.subs {
		entries = append(entries, CronHandoffEntry{
			RefID:    sub.ref.ID,
			ActorID:  sub.actorID,
			Schedule: sub.ref.Schedule,
			Tag:      sub.tag,
			Payload:  sub.payload,
		})
	}
	return &CronHandoffState{Entries: entries}
}

// restoreFromHandoff re-creates subscriptions from handoff state.
func (cs *cronBrokerService) restoreFromHandoff(hs *CronHandoffState) {
	for _, entry := range hs.Entries {
		sched, err := cs.parser.Parse(entry.Schedule)
		if err != nil {
			continue
		}

		ref := CronRef{
			ID:       entry.RefID,
			Schedule: entry.Schedule,
			ActorID:  entry.ActorID,
		}

		// Resolve subscriber PID. For local actors, look up by actor ID.
		// For remote actors (singleton handoff), construct from stored actorID.
		pid := PID{ActorID: entry.ActorID, Generation: 1}
		if cs.runtime.nodeID != "" {
			pid.Namespace = cs.runtime.nodeID
		}

		cronPID := PID{Namespace: pid.Namespace, ActorID: cs.cronID, Generation: 1}
		monRef := cs.runtime.Monitor(cronPID, pid)

		subCtx, cancel := context.WithCancel(context.Background())
		sub := &cronSubscription{
			ref:        ref,
			subscriber: pid,
			actorID:    entry.ActorID,
			schedule:   sched,
			payload:    entry.Payload,
			tag:        entry.Tag,
			monitorRef: monRef,
			cancel:     cancel,
		}

		cs.mu.Lock()
		cs.subs[ref.ID] = sub
		if cs.byActor[entry.ActorID] == nil {
			cs.byActor[entry.ActorID] = make(map[string]struct{})
		}
		cs.byActor[entry.ActorID][ref.ID] = struct{}{}
		cs.mu.Unlock()

		go cs.runSchedule(subCtx, sub)
	}
}

func (cs *cronBrokerService) replyAsk(ctx context.Context, requestID string, replyTo PID, payload CronCommandAck) {
	_ = cs.runtime.SendPID(ctx, replyTo, AskReplyEnvelope{
		RequestID: requestID,
		Payload:   payload,
		RepliedAt: cs.runtime.now(),
	})
}
