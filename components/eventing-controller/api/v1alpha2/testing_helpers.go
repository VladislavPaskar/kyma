package v1alpha2

var (
	ProtocolSettingsContentModeBinary = "BINARY"
	DefaultName                       = "test"
	DefaultNamespace                  = "test-namespace"
	DefaultSink                       = "https://svc2.test.local"
	DefaultID                         = "id"
	DefaultMaxInFlight                = 10
	DefaultStatusReady                = true
	DefaultConditions                 = []Condition{
		{
			Type:   ConditionSubscriptionActive,
			Status: "true",
		},
		{
			Type:   ConditionSubscribed,
			Status: "false",
		}}
)
