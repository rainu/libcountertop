package actions

// DeliveryResult defers a destructive follow-up until the transport has written
// its correlated acceptance receipt. This is not proof of application completion
// or end-to-end peer acknowledgment. Failed/uncertain writes must pass false.
type DeliveryResult interface{ ActionResultDelivered(bool) }

func ResultDelivered(answer any, ok bool) {
	if result, yes := answer.(DeliveryResult); yes {
		result.ActionResultDelivered(ok)
	}
}
