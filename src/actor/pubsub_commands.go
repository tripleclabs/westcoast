package actor

func isBrokerCommand(payload any) bool {
	switch payload.(type) {
	case BrokerSubscribeCommand, BrokerUnsubscribeCommand, BrokerPublishCommand:
		return true
	default:
		return false
	}
}
