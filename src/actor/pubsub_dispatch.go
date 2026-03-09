package actor

func classifyPublishResult(unreachable, total int) (BrokerOutcomeType, string) {
	if unreachable == 0 {
		return BrokerOutcomePublishSuccess, ""
	}
	if unreachable < total {
		return BrokerOutcomePublishPartialDelivery, "subscriber_unreachable"
	}
	return BrokerOutcomeTargetUnreachable, "subscriber_unreachable"
}
