package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func Test_Conversion(t *testing.T) {
	type TestCase struct {
		name             string
		alpha1Sub        *Subscription
		alpha2Sub        *v1alpha2.Subscription
		wantErrMsgV1toV2 string
		wantErrMsgV2toV1 string
	}

	testCases := []TestCase{
		{
			name: "Converting NATS Subscription with empty Filters",
			alpha1Sub: newDefaultSubscription(
				WithEmptyFilter(),
			),
			alpha2Sub: newV2DefaultSubscription(),
		},
		{
			name: "Converting NATS Subscription with multiple source which should result in a conversion error",
			alpha1Sub: newDefaultSubscription(
				WithFilter("app", OrderUpdatedEventType),
				WithFilter("", OrderDeletedEventTypeNonClean),
			),
			alpha2Sub:        newV2DefaultSubscription(),
			wantErrMsgV1toV2: errorMultipleSourceMsg,
		},
		{
			name: "Converting NATS Subscription with non-convertable maxInFlight in the config which should result in a conversion error",
			alpha1Sub: newDefaultSubscription(
				WithFilter("", OrderUpdatedEventType),
			),
			alpha2Sub: newV2DefaultSubscription(
				v2WithMaxInFlight("nonint"),
			),
			wantErrMsgV2toV1: "strconv.Atoi: parsing \"nonint\": invalid syntax",
		},
		{
			name: "Converting NATS Subscription with Filters",
			alpha1Sub: newDefaultSubscription(
				WithFilter(EventSource, OrderCreatedEventType),
				WithFilter(EventSource, OrderUpdatedEventType),
				WithFilter(EventSource, OrderDeletedEventTypeNonClean),
				WithStatusCleanEventTypes([]string{
					OrderCreatedEventType,
					OrderUpdatedEventType,
					OrderDeletedEventType,
				}),
			),
			alpha2Sub: newV2DefaultSubscription(
				v2WithSource(EventSource),
				v2WithTypes([]string{
					OrderCreatedEventType,
					OrderUpdatedEventType,
					OrderDeletedEventTypeNonClean,
				}),
				v2WithStatusTypes([]v1alpha2.EventType{
					{
						OriginalType: OrderCreatedEventType,
						CleanType:    OrderCreatedEventType,
					},
					{
						OriginalType: OrderUpdatedEventType,
						CleanType:    OrderUpdatedEventType,
					},
					{
						OriginalType: OrderDeletedEventTypeNonClean,
						CleanType:    OrderDeletedEventType,
					},
				}),
				v2WithStatusJetStreamTypes([]v1alpha2.JetStreamTypes{
					{
						OriginalType: OrderCreatedEventType,
						ConsumerName: "",
					},
					{
						OriginalType: OrderUpdatedEventType,
						ConsumerName: "",
					},
					{
						OriginalType: OrderDeletedEventTypeNonClean,
						ConsumerName: "",
					},
				}),
			),
		},
		{
			name: "Converting BEB Subscription",
			alpha1Sub: newDefaultSubscription(
				WithProtocolBEB(),
				withWebhookAuthForBEB(),
				WithFilter(EventSource, OrderCreatedEventType),
				WithFilter(EventSource, OrderUpdatedEventType),
				WithFilter(EventSource, OrderDeletedEventTypeNonClean),
				WithStatusCleanEventTypes([]string{
					OrderCreatedEventType,
					OrderUpdatedEventType,
					OrderDeletedEventType,
				}),
				WithBEBStatusFields(),
			),
			alpha2Sub: newV2DefaultSubscription(
				v2WithSource(EventSource),
				v2WithTypes([]string{
					OrderCreatedEventType,
					OrderUpdatedEventType,
					OrderDeletedEventTypeNonClean,
				}),
				v2WithProtocolBEB(),
				v2WithWebhookAuthForBEB(),
				v2WithStatusTypes([]v1alpha2.EventType{
					{
						OriginalType: OrderCreatedEventType,
						CleanType:    OrderCreatedEventType,
					},
					{
						OriginalType: OrderUpdatedEventType,
						CleanType:    OrderUpdatedEventType,
					},
					{
						OriginalType: OrderDeletedEventTypeNonClean,
						CleanType:    OrderDeletedEventType,
					},
				}),
				v2WithBEBStatusFields(),
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			//WHEN
			t.Run("Test v1 to v2 conversion", func(t *testing.T) {
				// skip the conversion if the backwards conversion cannot succeed
				if testCase.wantErrMsgV2toV1 != "" {
					return
				}
				convertedV1Alpha2 := &v1alpha2.Subscription{}
				err := v1ToV2(testCase.alpha1Sub, convertedV1Alpha2)
				if err != nil && testCase.wantErrMsgV1toV2 != "" {
					require.Equal(t, err.Error(), testCase.wantErrMsgV1toV2)
				} else {
					require.NoError(t, err)
					v1ToV2Assertions(t, testCase.alpha2Sub, convertedV1Alpha2)
				}
			})

			// test ConvertFrom
			t.Run("Test v2 to v1 conversion", func(t *testing.T) {
				// skip the backwards conversion if the initial one cannot succeed
				if testCase.wantErrMsgV1toV2 != "" {
					return
				}
				convertedV1Alpha1 := &Subscription{}
				err := v2ToV1(convertedV1Alpha1, testCase.alpha2Sub)
				if err != nil && testCase.wantErrMsgV2toV1 != "" {
					require.Equal(t, err.Error(), testCase.wantErrMsgV2toV1)
				} else {
					require.NoError(t, err)
					v2ToV1Assertions(t, testCase.alpha1Sub, convertedV1Alpha1)
				}

			})
		})
	}
}

func v1ToV2Assertions(t *testing.T, wantSub, convertedSub *v1alpha2.Subscription) {
	assert.Equal(t, wantSub.ObjectMeta, convertedSub.ObjectMeta)

	// Spec
	assert.Equal(t, wantSub.Spec.ID, convertedSub.Spec.ID)
	assert.Equal(t, wantSub.Spec.Sink, convertedSub.Spec.Sink)
	assert.Equal(t, wantSub.Spec.TypeMatching, convertedSub.Spec.TypeMatching)
	assert.Equal(t, wantSub.Spec.Source, convertedSub.Spec.Source)
	assert.Equal(t, wantSub.Spec.Types, convertedSub.Spec.Types)
	assert.Equal(t, wantSub.Spec.Config, convertedSub.Spec.Config)

	// Status
	assert.Equal(t, wantSub.Status.Ready, convertedSub.Status.Ready)
	assert.Equal(t, wantSub.Status.Conditions, convertedSub.Status.Conditions)
	assert.Equal(t, wantSub.Status.Types, convertedSub.Status.Types)

	assert.Equal(t, wantSub.Status.Backend, convertedSub.Status.Backend)
}

func v2ToV1Assertions(t *testing.T, wantSub, convertedSub *Subscription) {
	assert.Equal(t, wantSub.ObjectMeta, convertedSub.ObjectMeta)

	// Spec
	assert.Equal(t, wantSub.Spec.ID, convertedSub.Spec.ID)
	assert.Equal(t, wantSub.Spec.Sink, convertedSub.Spec.Sink)
	assert.Equal(t, wantSub.Spec.Protocol, convertedSub.Spec.Protocol)
	assert.Equal(t, wantSub.Spec.ProtocolSettings, convertedSub.Spec.ProtocolSettings)

	assert.Equal(t, wantSub.Spec.Filter, convertedSub.Spec.Filter)
	assert.Equal(t, wantSub.Spec.Config, convertedSub.Spec.Config)

	// Status
	assert.Equal(t, wantSub.Status.Ready, convertedSub.Status.Ready)
	assert.Equal(t, wantSub.Status.Conditions, convertedSub.Status.Conditions)
	assert.Equal(t, wantSub.Status.CleanEventTypes, convertedSub.Status.CleanEventTypes)

	// BEB fields
	assert.Equal(t, wantSub.Status.Ev2hash, convertedSub.Status.Ev2hash)
	assert.Equal(t, wantSub.Status.Emshash, convertedSub.Status.Emshash)
	assert.Equal(t, wantSub.Status.ExternalSink, convertedSub.Status.ExternalSink)
	assert.Equal(t, wantSub.Status.FailedActivation, convertedSub.Status.FailedActivation)
	assert.Equal(t, wantSub.Status.APIRuleName, convertedSub.Status.APIRuleName)
	assert.Equal(t, wantSub.Status.EmsSubscriptionStatus, convertedSub.Status.EmsSubscriptionStatus)

	assert.Equal(t, wantSub.Status.Config, convertedSub.Status.Config)
}
