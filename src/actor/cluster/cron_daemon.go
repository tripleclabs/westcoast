package cluster

import "github.com/tripleclabs/westcoast/src/actor"

const defaultCronDaemonMailboxCapacity = 65536

// RegisterCronDaemon registers a per-host cron timer as a daemon actor.
// On each matching node, a cron timer actor is created that local actors
// can subscribe to via CronSubscribe. If cronID is empty, the default
// cron actor ID is used.
func (dm *DaemonSetManager) RegisterCronDaemon(cronID string, placement NodeMatcher) {
	if cronID == "" {
		cronID = actor.DefaultCronActorID
	}
	handler := dm.runtime.CronHandler(cronID)
	dm.Register(DaemonSpec{
		Name:      cronID,
		Handler:   handler,
		Options:   []actor.ActorOption{actor.WithMailboxCapacity(defaultCronDaemonMailboxCapacity)},
		Placement: placement,
	})
}
