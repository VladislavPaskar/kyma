package v1alpha1

import (
	"fmt"

	"github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha2"
	"github.com/kyma-project/kyma/components/eventing-controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EventSource                   = "source"
	OrderCreatedEventType         = "prefix." + "noapp." + "order.created.v1"
	OrderUpdatedEventType         = "prefix." + "app." + "order.updated.v1"
	OrderDeletedEventType         = "prefix." + "noapp." + "order.deleted.v1"
	OrderDeletedEventTypeNonClean = "prefix." + "noapp." + "order.deleted_&.v1"
)

const (
	DefaultName        = "test"
	DefaultNamespace   = "test-namespace"
	DefaultSink        = "https://svc2.test.local"
	DefaultID          = "id"
	DefaultMaxInFlight = 10
	DefaultStatusReady = true
)

var (
	DefaultConditions = []v1alpha2.Condition{
		{
			Type:   v1alpha2.ConditionSubscriptionActive,
			Status: "true",
		},
		{
			Type:   v1alpha2.ConditionSubscribed,
			Status: "false",
		}}
)

// +kubebuilder:object:generate=false
type SubscriptionOpt func(subscription *Subscription)

func newDefaultSubscription(opts ...SubscriptionOpt) *Subscription {
	var defaultConditions []Condition
	for _, condition := range DefaultConditions {
		defaultConditions = append(defaultConditions, ConditionV2ToV1(condition))
	}
	newSub := &Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.kyma-project.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultName,
			Namespace: DefaultNamespace,
		},
		Spec: SubscriptionSpec{
			Sink:   DefaultSink,
			ID:     DefaultID,
			Config: &SubscriptionConfig{MaxInFlightMessages: DefaultMaxInFlight},
		},
		Status: SubscriptionStatus{
			Conditions: defaultConditions,
			Ready:      DefaultStatusReady,
			Config:     &SubscriptionConfig{MaxInFlightMessages: DefaultMaxInFlight},
		},
	}
	for _, o := range opts {
		o(newSub)
	}

	// remove nats specific field in beb case
	if newSub.Status.EmsSubscriptionStatus != nil {
		newSub.Spec.Config = nil
		newSub.Status.Config = nil
	}

	return newSub
}

func WithStatus(status bool) SubscriptionOpt {
	return func(sub *Subscription) {
		sub.Status.Ready = status
	}
}

func WithStatusCleanEventTypes(cleanEventTypes []string) SubscriptionOpt {
	return func(sub *Subscription) {
		if cleanEventTypes == nil {
			sub.Status.InitializeCleanEventTypes()
		} else {
			sub.Status.CleanEventTypes = cleanEventTypes
		}
	}
}

func withWebhookAuthForBEB() SubscriptionOpt {
	return func(s *Subscription) {
		s.Spec.Protocol = "BEB"
		s.Spec.ProtocolSettings = &ProtocolSettings{
			ContentMode: func() *string {
				contentMode := ProtocolSettingsContentModeBinary
				return &contentMode
			}(),
			Qos: func() *string {
				qos := "true"
				return &qos
			}(),
			ExemptHandshake: utils.BoolPtr(true),
			WebhookAuth: &WebhookAuth{
				Type:         "oauth2",
				GrantType:    "client_credentials",
				ClientID:     "xxx",
				ClientSecret: "xxx",
				TokenURL:     "https://oauth2.xxx.com/oauth2/token",
				Scope:        []string{"guid-identifier", "root"},
			},
		}
	}
}

func WithProtocolBEB() SubscriptionOpt {
	return func(s *Subscription) {
		s.Spec.Protocol = "BEB"
	}
}

func WithBEBStatusFields() SubscriptionOpt {
	return func(s *Subscription) {
		s.Status.Ev2hash = 123
		s.Status.ExternalSink = "testlink.com"
		s.Status.FailedActivation = "123156464672"
		s.Status.APIRuleName = "APIRule"
		s.Status.EmsSubscriptionStatus = &EmsSubscriptionStatus{
			SubscriptionStatus:       "not active",
			SubscriptionStatusReason: "reason",
			LastSuccessfulDelivery:   "",
			LastFailedDelivery:       "1345613234",
			LastFailedDeliveryReason: "failed",
		}
	}
}

// WithWebhookForNATS is a SubscriptionOpt for creating a Subscription with a webhook set to the NATS protocol.
func WithWebhookForNATS() SubscriptionOpt {
	return func(s *Subscription) {
		s.Spec.Protocol = "NATS"
		s.Spec.ProtocolSettings = &ProtocolSettings{}
	}
}

// WithFilter is a SubscriptionOpt for creating a Subscription with a specific event type filter,
// that itself gets created from the passed eventSource and eventType.
func WithFilter(eventSource, eventType string) SubscriptionOpt {
	return func(subscription *Subscription) { AddFilter(eventSource, eventType, subscription) }
}

// AddFilter creates a new Filter from eventSource and eventType and adds it to the subscription.
func AddFilter(eventSource, eventType string, subscription *Subscription) {
	if subscription.Spec.Filter == nil {
		subscription.Spec.Filter = &BEBFilters{
			Filters: []*BEBFilter{},
		}
	}

	filter := &BEBFilter{
		EventSource: &Filter{
			Type:     "exact",
			Property: "source",
			Value:    eventSource,
		},
		EventType: &Filter{
			Type:     "exact",
			Property: "type",
			Value:    eventType,
		},
	}

	subscription.Spec.Filter.Filters = append(subscription.Spec.Filter.Filters, filter)
}

// WithEmptyFilter is a SubscriptionOpt for creating a subscription with an empty event type filter.
// Note that this is different from setting Filter to nil.
func WithEmptyFilter() SubscriptionOpt {
	return func(subscription *Subscription) {
		subscription.Spec.Filter = &BEBFilters{
			Filters: []*BEBFilter{},
		}
		subscription.Status.InitializeCleanEventTypes()
	}
}

type v2SubscriptionOpt func(subscription *v1alpha2.Subscription)

func v2WithMaxInFlight(maxInFlight string) v2SubscriptionOpt {
	return func(sub *v1alpha2.Subscription) {
		sub.Spec.Config = map[string]string{
			v1alpha2.MaxInFlightMessages: fmt.Sprint(maxInFlight),
		}
	}
}

func v2WithSource(source string) v2SubscriptionOpt {
	return func(sub *v1alpha2.Subscription) {
		sub.Spec.Source = source
	}
}

func v2WithTypes(types []string) v2SubscriptionOpt {
	return func(sub *v1alpha2.Subscription) {
		sub.Spec.Types = types
	}
}
func v2WithStatusJetStreamTypes(types []v1alpha2.JetStreamTypes) v2SubscriptionOpt {
	return func(sub *v1alpha2.Subscription) {
		sub.Status.Backend.Types = types
	}
}

func v2WithStatusTypes(statusTypes []v1alpha2.EventType) v2SubscriptionOpt {
	return func(sub *v1alpha2.Subscription) {
		if statusTypes == nil {
			sub.Status.InitializeEventTypes()
			return
		}
		sub.Status.Types = statusTypes
	}
}

func v2WithWebhookAuthForBEB() v2SubscriptionOpt {
	return func(s *v1alpha2.Subscription) {
		s.Spec.Config = map[string]string{
			v1alpha2.Protocol:                        "BEB",
			v1alpha2.ProtocolSettingsContentMode:     ProtocolSettingsContentModeBinary,
			v1alpha2.ProtocolSettingsExemptHandshake: "true",
			v1alpha2.ProtocolSettingsQos:             "true",
			v1alpha2.WebhookAuthType:                 "oauth2",
			v1alpha2.WebhookAuthGrantType:            "client_credentials",
			v1alpha2.WebhookAuthClientID:             "xxx",
			v1alpha2.WebhookAuthClientSecret:         "xxx",
			v1alpha2.WebhookAuthTokenURL:             "https://oauth2.xxx.com/oauth2/token",
			v1alpha2.WebhookAuthScope:                "guid-identifier,root",
		}
	}
}

func v2WithProtocolBEB() v2SubscriptionOpt {
	return func(s *v1alpha2.Subscription) {
		if s.Spec.Config == nil {
			s.Spec.Config = map[string]string{}
		}
		s.Spec.Config[v1alpha2.Protocol] = "BEB"
	}
}

func v2WithBEBStatusFields() v2SubscriptionOpt {
	return func(s *v1alpha2.Subscription) {
		s.Status.Backend.Ev2hash = 123
		s.Status.Backend.ExternalSink = "testlink.com"
		s.Status.Backend.FailedActivation = "123156464672"
		s.Status.Backend.APIRuleName = "APIRule"
		s.Status.Backend.EmsSubscriptionStatus = &v1alpha2.EmsSubscriptionStatus{
			Status:                   "not active",
			StatusReason:             "reason",
			LastSuccessfulDelivery:   "",
			LastFailedDelivery:       "1345613234",
			LastFailedDeliveryReason: "failed",
		}
	}
}
func newV2DefaultSubscription(opts ...v2SubscriptionOpt) *v1alpha2.Subscription {
	newSub := &v1alpha2.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: "eventing.kyma-project.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultName,
			Namespace: DefaultNamespace,
		},
		Spec: v1alpha2.SubscriptionSpec{
			TypeMatching: v1alpha2.TypeMatchingExact,
			Sink:         DefaultSink,
			ID:           DefaultID,
			Config: map[string]string{
				v1alpha2.MaxInFlightMessages: fmt.Sprint(DefaultMaxInFlight),
			},
		},
		Status: v1alpha2.SubscriptionStatus{
			Ready:      DefaultStatusReady,
			Conditions: DefaultConditions,
		},
	}
	for _, o := range opts {
		o(newSub)
	}

	return newSub
}
