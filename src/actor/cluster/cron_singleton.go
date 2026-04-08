package cluster

import "github.com/tripleclabs/westcoast/src/actor"

const defaultCronSingletonMailboxCapacity = 65536

// RegisterCronSingleton registers a cluster-wide cron timer singleton.
// Exactly one instance runs across the cluster at any time. On leadership
// change, all cron subscriptions are transferred to the new leader via
// the singleton handoff protocol.
//
// If cronID is empty, the default singleton cron actor ID is used.
// codec must be provided to enable handoff state serialization.
func (sm *SingletonManager) RegisterCronSingleton(cronID string, placement NodeMatcher, codec Codec) {
	if cronID == "" {
		cronID = actor.SingletonCronActorID
	}

	// Register handoff types with codec for gob encoding.
	if codec != nil {
		codec.Register(actor.CronHandoffState{})
		codec.Register(actor.CronHandoffEntry{})
		codec.Register(actor.CronTickMessage{})
		codec.Register(actor.CronSubscribeCommand{})
		codec.Register(actor.CronUnsubscribeCommand{})
		codec.Register(actor.CronUnsubscribeAllCommand{})
		codec.Register(actor.CronListCommand{})
		codec.Register(actor.CronCommandAck{})
		codec.Register(actor.CronRef{})
	}

	handler := sm.runtime.CronHandler(cronID)

	sm.Register(SingletonSpec{
		Name:      cronID,
		Handler:   handler,
		Options:   []actor.ActorOption{actor.WithMailboxCapacity(defaultCronSingletonMailboxCapacity)},
		Placement: placement,
		OnHandoff: func(state any) any {
			// Pass through the cronHandoffState to the new leader.
			return state
		},
	})
}
