package eventmesh

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	backendutilsv2 "github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/utils/v2"

	apigatewayv1beta1 "github.com/kyma-incubator/api-gateway/api/v1beta1"
	kymalogger "github.com/kyma-project/kyma/common/logging/logger"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/beb"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/cleaner"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventingv1alpha2 "github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha2"
	eventinglogger "github.com/kyma-project/kyma/components/eventing-controller/logger"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/eventmesh/mocks"
	sink "github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/sink/v2"
	backendutils "github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/utils"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/ems/api/events/types"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/env"
	reconcilertesting "github.com/kyma-project/kyma/components/eventing-controller/testing/v2"
)

const (
	domain = "domain.com"
)

// TestReconciler_Reconcile tests the return values of the Reconcile() method of the reconciler.
// This is important, as it dictates whether the reconciliation should be requeued by Controller Runtime,
// and if so with how much initial delay. Returning error or a `Result{Requeue: true}` would cause the reconciliation to be requeued.
// Everything else is mocked since we are only interested in the logic of the Reconcile method and not the reconciler dependencies.
func TestReconciler_Reconcile(t *testing.T) {
	ctx := context.Background()
	req := require.New(t)

	// A subscription with the correct Finalizer, conditions and status ready for reconciliation with the backend.
	testSub := reconcilertesting.NewSubscription("sub1", "test",
		reconcilertesting.WithConditions(eventingv1alpha2.MakeSubscriptionConditions()),
		reconcilertesting.WithFinalizers([]string{eventingv1alpha2.Finalizer}),
		reconcilertesting.WithDefaultSource(),
		reconcilertesting.WithEventType(reconcilertesting.OrderCreatedEventType),
		reconcilertesting.WithValidSink("test", "test-svc"),
		reconcilertesting.WithEmsSubscriptionStatus(string(types.SubscriptionStatusActive)),
	)
	// A subscription marked for deletion.
	testSubUnderDeletion := reconcilertesting.NewSubscription("sub2", "test",
		reconcilertesting.WithNonZeroDeletionTimestamp(),
		reconcilertesting.WithFinalizers([]string{eventingv1alpha2.Finalizer}),
		reconcilertesting.WithDefaultSource(),
		reconcilertesting.WithEventType(reconcilertesting.OrderCreatedEventType),
	)

	// A subscription with the correct Finalizer, conditions and status Paused for reconciliation with the backend.
	testSubPaused := reconcilertesting.NewSubscription("sub3", "test",
		reconcilertesting.WithConditions(eventingv1alpha2.MakeSubscriptionConditions()),
		reconcilertesting.WithFinalizers([]string{eventingv1alpha2.Finalizer}),
		reconcilertesting.WithDefaultSource(),
		reconcilertesting.WithEventType(reconcilertesting.OrderCreatedEventType),
		reconcilertesting.WithValidSink("test", "test-svc"),
		reconcilertesting.WithEmsSubscriptionStatus(string(types.SubscriptionStatusPaused)),
	)

	backendSyncErr := errors.New("backend sync error")
	backendDeleteErr := errors.New("backend delete error")
	validatorErr := errors.New("invalid sink")
	happyValidator := sink.ValidatorFunc(func(s *eventingv1alpha2.Subscription) error { return nil })
	unhappyValidator := sink.ValidatorFunc(func(s *eventingv1alpha2.Subscription) error { return validatorErr })

	var testCases = []struct {
		name                 string
		givenSubscription    *eventingv1alpha2.Subscription
		givenReconcilerSetup func() *Reconciler
		wantReconcileResult  ctrl.Result
		wantReconcileError   error
	}{
		{
			name:              "Return nil and default Result{} when there is no error from the reconciler dependencies",
			givenSubscription: testSub,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t, testSub)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				te.backend.On("SyncSubscription", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, happyValidator)
			},
			wantReconcileResult: ctrl.Result{},
			wantReconcileError:  nil,
		},
		{
			name:              "Return nil and default Result{} when the subscription does not exist on the cluster",
			givenSubscription: testSub,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, unhappyValidator)
			},
			wantReconcileResult: ctrl.Result{},
			wantReconcileError:  nil,
		},
		{
			name:              "Return error and default Result{} when backend sync returns error",
			givenSubscription: testSub,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t, testSub)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				te.backend.On("SyncSubscription", mock.Anything, mock.Anything, mock.Anything).Return(false, backendSyncErr)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, happyValidator)
			},
			wantReconcileResult: ctrl.Result{},
			wantReconcileError:  backendSyncErr,
		},
		{
			name:              "Return error and default Result{} when backend delete returns error",
			givenSubscription: testSubUnderDeletion,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t, testSubUnderDeletion)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				te.backend.On("DeleteSubscription", mock.Anything).Return(backendDeleteErr)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, happyValidator)
			},
			wantReconcileResult: ctrl.Result{},
			wantReconcileError:  backendDeleteErr,
		},
		{
			name:              "Return error and default Result{} when validator returns error",
			givenSubscription: testSub,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t, testSub)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, unhappyValidator)
			},
			wantReconcileResult: ctrl.Result{},
			wantReconcileError:  validatorErr,
		},
		{
			name:              "Return nil and RequeueAfter when the EventMesh subscription is Paused",
			givenSubscription: testSubPaused,
			givenReconcilerSetup: func() *Reconciler {
				te := setupTestEnvironment(t, testSubPaused)
				te.backend.On("Initialize", mock.Anything).Return(nil)
				te.backend.On("SyncSubscription", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
				return NewReconciler(ctx, te.fakeClient, te.logger, te.recorder, te.cfg, te.cleaner, te.backend, te.credentials, te.mapper, happyValidator)
			},
			wantReconcileResult: ctrl.Result{
				RequeueAfter: requeueAfterDuration,
			},
			wantReconcileError: nil,
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			reconciler := tc.givenReconcilerSetup()
			r := ctrl.Request{NamespacedName: k8stypes.NamespacedName{
				Namespace: tc.givenSubscription.Namespace,
				Name:      tc.givenSubscription.Name,
			}}
			res, err := reconciler.Reconcile(context.Background(), r)
			req.Equal(res, tc.wantReconcileResult)
			req.Equal(err, tc.wantReconcileError)
		})
	}
}

func Test_replaceStatusCondition(t *testing.T) {
	var testCases = []struct {
		name              string
		giveSubscription  *eventingv1alpha2.Subscription
		giveCondition     eventingv1alpha2.Condition
		wantStatusChanged bool
		wantStatus        *eventingv1alpha2.SubscriptionStatus
		wantReady         bool
	}{
		{
			name: "Updating a condition marks the status as changed",
			giveSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace",
					reconcilertesting.WithNotCleanSource(),
					reconcilertesting.WithNotCleanType())
				sub.Status.InitializeConditions()
				return sub
			}(),
			giveCondition: func() eventingv1alpha2.Condition {
				return eventingv1alpha2.MakeCondition(eventingv1alpha2.ConditionSubscribed, eventingv1alpha2.ConditionReasonSubscriptionCreated, corev1.ConditionTrue, "")
			}(),
			wantStatusChanged: true,
			wantReady:         false,
		},
		{
			name: "All conditions true means status is ready",
			giveSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace",
					reconcilertesting.WithNotCleanSource(),
					reconcilertesting.WithNotCleanType(),
					reconcilertesting.WithWebhookAuthForBEB())
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark all conditions as true
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: metav1.Now(),
						Status:             corev1.ConditionTrue,
					},
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: metav1.Now(),
						Status:             corev1.ConditionTrue,
					},
				}
				return sub
			}(),
			giveCondition: func() eventingv1alpha2.Condition {
				return eventingv1alpha2.MakeCondition(eventingv1alpha2.ConditionSubscribed, eventingv1alpha2.ConditionReasonSubscriptionCreated, corev1.ConditionTrue, "")
			}(),
			wantStatusChanged: true, // readiness changed
			wantReady:         true, // all conditions are true
		},
	}

	r := Reconciler{}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			sub := tt.giveSubscription
			condition := tt.giveCondition
			statusChanged := r.replaceStatusCondition(sub, condition)
			assert.Equal(t, tt.wantStatusChanged, statusChanged)
			assert.Contains(t, sub.Status.Conditions, condition)
			assert.Equal(t, tt.wantReady, sub.Status.Ready)
		})
	}
}

func Test_getRequiredConditions(t *testing.T) {
	var emptySubscriptionStatus eventingv1alpha2.SubscriptionStatus
	emptySubscriptionStatus.InitializeConditions()
	expectedConditions := emptySubscriptionStatus.Conditions

	testCases := []struct {
		name                   string
		subscriptionConditions []eventingv1alpha2.Condition
		wantConditions         []eventingv1alpha2.Condition
	}{
		{
			name:                   "should get expected conditions if the subscription has no conditions",
			subscriptionConditions: []eventingv1alpha2.Condition{},
			wantConditions:         expectedConditions,
		},
		{
			name: "should get subscription conditions if the all the expected conditions are present",
			subscriptionConditions: []eventingv1alpha2.Condition{
				{Type: eventingv1alpha2.ConditionSubscribed, Status: corev1.ConditionTrue},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionAPIRuleStatus, Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionWebhookCallStatus, Status: corev1.ConditionFalse},
			},
			wantConditions: []eventingv1alpha2.Condition{
				{Type: eventingv1alpha2.ConditionSubscribed, Status: corev1.ConditionTrue},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionAPIRuleStatus, Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionWebhookCallStatus, Status: corev1.ConditionFalse},
			},
		},
		{
			name: "should get latest conditions Status compared to the expected condition status",
			subscriptionConditions: []eventingv1alpha2.Condition{
				{Type: eventingv1alpha2.ConditionSubscribed, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
			},
			wantConditions: []eventingv1alpha2.Condition{
				{Type: eventingv1alpha2.ConditionSubscribed, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionAPIRuleStatus, Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionWebhookCallStatus, Status: corev1.ConditionUnknown},
			},
		},
		{
			name: "should get rid of unwanted conditions in the subscription, if present",
			subscriptionConditions: []eventingv1alpha2.Condition{
				{Type: "Fake Condition Type", Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
			},
			wantConditions: []eventingv1alpha2.Condition{
				{Type: eventingv1alpha2.ConditionSubscribed, Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionSubscriptionActive, Status: corev1.ConditionFalse},
				{Type: eventingv1alpha2.ConditionAPIRuleStatus, Status: corev1.ConditionUnknown},
				{Type: eventingv1alpha2.ConditionWebhookCallStatus, Status: corev1.ConditionUnknown},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotConditions := getRequiredConditions(tc.subscriptionConditions, expectedConditions)
			assert.True(t, eventingv1alpha2.ConditionsEquals(gotConditions, tc.wantConditions))
			assert.Len(t, gotConditions, len(expectedConditions))
		})
	}
}

func Test_syncConditionSubscribed(t *testing.T) {
	currentTime := metav1.Now()
	errorMessage := "error message"
	var testCases = []struct {
		name              string
		givenSubscription *eventingv1alpha2.Subscription
		givenError        error
		wantCondition     eventingv1alpha2.Condition
	}{
		{
			name: "should replace condition with status false",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark ConditionSubscribed conditions as true
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
				}
				return sub
			}(),
			givenError: errors.New(errorMessage),
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionSubscribed,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionFalse,
				Reason:             eventingv1alpha2.ConditionReasonSubscriptionCreationFailed,
				Message:            errorMessage,
			},
		},
		{
			name: "should replace condition with status true",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark ConditionSubscribed conditions as false
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
				}
				return sub
			}(),
			givenError: nil,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionSubscribed,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionTrue,
				Reason:             eventingv1alpha2.ConditionReasonSubscriptionCreated,
				Message:            "EventMesh subscription name is: some-namef73aa86661706ae6ba5acf1d32821ce318051d0e",
			},
		},
	}

	r := Reconciler{
		nameMapper: backendutils.NewBEBSubscriptionNameMapper(domain, beb.MaxBEBSubscriptionNameLength),
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sub := tc.givenSubscription

			// when
			r.syncConditionSubscribed(sub, tc.givenError)

			// then
			newCondition := sub.Status.FindCondition(tc.wantCondition.Type)
			assert.NotNil(t, newCondition)
			assert.True(t, eventingv1alpha2.ConditionEquals(*newCondition, tc.wantCondition))
		})
	}
}

func Test_syncConditionSubscriptionActive(t *testing.T) {
	currentTime := metav1.Now()

	var testCases = []struct {
		name              string
		givenSubscription *eventingv1alpha2.Subscription
		givenIsSubscribed bool
		wantCondition     eventingv1alpha2.Condition
	}{
		{
			name: "should replace condition with status false",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: "Paused",
				}

				// mark ConditionSubscriptionActive conditions as true
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
				}
				return sub
			}(),
			givenIsSubscribed: false,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionSubscriptionActive,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionFalse,
				Reason:             eventingv1alpha2.ConditionReasonSubscriptionNotActive,
				Message:            "Waiting for subscription to be active",
			},
		},
		{
			name: "should replace condition with status true",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{}

				// mark ConditionSubscriptionActive conditions as false
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
				}
				return sub
			}(),
			givenIsSubscribed: true,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionSubscriptionActive,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionTrue,
				Reason:             eventingv1alpha2.ConditionReasonSubscriptionActive,
			},
		},
	}

	logger, err := eventinglogger.New(string(kymalogger.JSON), string(kymalogger.INFO))
	if err != nil {
		t.Fatalf(`failed to initiate logger, %v`, err)
	}

	r := Reconciler{
		nameMapper: backendutils.NewBEBSubscriptionNameMapper(domain, beb.MaxBEBSubscriptionNameLength),
		logger:     logger,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sub := tc.givenSubscription
			log := backendutilsv2.LoggerWithSubscription(r.namedLogger(), sub)

			// when
			r.syncConditionSubscriptionActive(sub, tc.givenIsSubscribed, log)

			// then
			newCondition := sub.Status.FindCondition(tc.wantCondition.Type)
			assert.NotNil(t, newCondition)
			assert.True(t, eventingv1alpha2.ConditionEquals(*newCondition, tc.wantCondition))
		})
	}
}

func Test_syncConditionWebhookCallStatus(t *testing.T) {
	currentTime := metav1.Now()

	var testCases = []struct {
		name              string
		givenSubscription *eventingv1alpha2.Subscription
		givenIsSubscribed bool
		wantCondition     eventingv1alpha2.Condition
	}{
		{
			name: "should replace condition with status false if it throws error to check lastDelivery",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark ConditionWebhookCallStatus conditions as true
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscriptionActive,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
					{
						Type:               eventingv1alpha2.ConditionWebhookCallStatus,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionTrue,
					},
				}
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   "invalid",
					LastFailedDelivery:       "invalid",
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			givenIsSubscribed: false,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionWebhookCallStatus,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionFalse,
				Reason:             eventingv1alpha2.ConditionReasonWebhookCallStatus,
				Message:            `failed to parse LastFailedDelivery: parsing time "invalid" as "2006-01-02T15:04:05Z07:00": cannot parse "invalid" as "2006"`,
			},
		},
		{
			name: "should replace condition with status false if lastDelivery was not okay",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark ConditionWebhookCallStatus conditions as false
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
					{
						Type:               eventingv1alpha2.ConditionWebhookCallStatus,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
				}
				// set EmsSubscriptionStatus
				// LastFailedDelivery is latest
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   time.Now().Format(time.RFC3339),
					LastFailedDelivery:       time.Now().Add(1 * time.Hour).Format(time.RFC3339),
					LastFailedDeliveryReason: "abc",
				}
				return sub
			}(),
			givenIsSubscribed: true,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionWebhookCallStatus,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionFalse,
				Reason:             eventingv1alpha2.ConditionReasonWebhookCallStatus,
				Message:            "abc",
			},
		},
		{
			name: "should replace condition with status true if lastDelivery was okay",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Ready = false

				// mark ConditionWebhookCallStatus conditions as false
				sub.Status.Conditions = []eventingv1alpha2.Condition{
					{
						Type:               eventingv1alpha2.ConditionSubscribed,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
					{
						Type:               eventingv1alpha2.ConditionWebhookCallStatus,
						LastTransitionTime: currentTime,
						Status:             corev1.ConditionFalse,
					},
				}
				// set EmsSubscriptionStatus
				// LastSuccessfulDelivery is latest
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
					LastFailedDelivery:       time.Now().Format(time.RFC3339),
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			givenIsSubscribed: true,
			wantCondition: eventingv1alpha2.Condition{
				Type:               eventingv1alpha2.ConditionWebhookCallStatus,
				LastTransitionTime: currentTime,
				Status:             corev1.ConditionTrue,
				Reason:             eventingv1alpha2.ConditionReasonWebhookCallStatus,
				Message:            "",
			},
		},
	}

	logger, err := eventinglogger.New(string(kymalogger.JSON), string(kymalogger.INFO))
	if err != nil {
		t.Fatalf(`failed to initiate logger, %v`, err)
	}

	r := Reconciler{
		logger: logger,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sub := tc.givenSubscription

			// when
			r.syncConditionWebhookCallStatus(sub)

			// then
			newCondition := sub.Status.FindCondition(tc.wantCondition.Type)
			assert.NotNil(t, newCondition)
			assert.True(t, eventingv1alpha2.ConditionEquals(*newCondition, tc.wantCondition))
		})
	}
}

func Test_checkStatusActive(t *testing.T) {
	currentTime := time.Now()
	testCases := []struct {
		name         string
		subscription *eventingv1alpha2.Subscription
		wantStatus   bool
		wantError    error
	}{
		{
			name: "should return active since the EmsSubscriptionStatus is active",
			subscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: string(types.SubscriptionStatusActive),
				}
				return sub
			}(),
			wantStatus: true,
			wantError:  nil,
		},
		{
			name: "should return active if the EmsSubscriptionStatus is active and the FailedActivation time is set",
			subscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Backend.FailedActivation = currentTime.Format(time.RFC3339)
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: string(types.SubscriptionStatusActive),
				}
				return sub
			}(),
			wantStatus: true,
			wantError:  nil,
		},
		{
			name: "should return not active if the EmsSubscriptionStatus is inactive",
			subscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: string(types.SubscriptionStatusPaused),
				}
				return sub
			}(),
			wantStatus: false,
			wantError:  nil,
		},
		{
			name: "should return not active if the EmsSubscriptionStatus is inactive and the the FailedActivation time is set",
			subscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Backend.FailedActivation = currentTime.Format(time.RFC3339)
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: string(types.SubscriptionStatusPaused),
				}
				return sub
			}(),
			wantStatus: false,
			wantError:  nil,
		},
		{
			name: "should error if timed out waiting after retrying",
			subscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				sub.Status.InitializeConditions()
				sub.Status.Backend.FailedActivation = currentTime.Add(time.Minute * -1).Format(time.RFC3339)
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					Status: string(types.SubscriptionStatusPaused),
				}
				return sub
			}(),
			wantStatus: false,
			wantError:  errors.New("timeout waiting for the subscription to be active: some-name"),
		},
	}

	r := Reconciler{}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, err := r.checkStatusActive(tc.subscription)
			assert.Equal(t, tc.wantStatus, gotStatus)
			if tc.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, tc.wantError, err)
			}
		})
	}
}

func Test_checkLastFailedDelivery(t *testing.T) {
	var testCases = []struct {
		name              string
		givenSubscription *eventingv1alpha2.Subscription
		wantResult        bool
		wantError         error
	}{
		{
			name: "should return false if there is no lastFailedDelivery",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   "",
					LastFailedDelivery:       "",
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			wantResult: false,
			wantError:  nil,
		},
		{
			name: "should return error if LastFailedDelivery is invalid",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   "",
					LastFailedDelivery:       "invalid",
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			wantResult: true,
			wantError:  errors.New(`failed to parse LastFailedDelivery: parsing time "invalid" as "2006-01-02T15:04:05Z07:00": cannot parse "invalid" as "2006"`),
		},
		{
			name: "should return error if LastFailedDelivery is valid but LastSuccessfulDelivery is invalid",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   "invalid",
					LastFailedDelivery:       time.Now().Format(time.RFC3339),
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			wantResult: true,
			wantError:  errors.New(`failed to parse LastSuccessfulDelivery: parsing time "invalid" as "2006-01-02T15:04:05Z07:00": cannot parse "invalid" as "2006"`),
		},
		{
			name: "should return true if last delivery of event was failed",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   time.Now().Format(time.RFC3339),
					LastFailedDelivery:       time.Now().Add(1 * time.Hour).Format(time.RFC3339),
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			wantResult: true,
			wantError:  nil,
		},
		{
			name: "should return false if last delivery of event was success",
			givenSubscription: func() *eventingv1alpha2.Subscription {
				sub := reconcilertesting.NewSubscription("some-name", "some-namespace")
				// set EmsSubscriptionStatus
				sub.Status.Backend.EmsSubscriptionStatus = &eventingv1alpha2.EmsSubscriptionStatus{
					LastSuccessfulDelivery:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
					LastFailedDelivery:       time.Now().Format(time.RFC3339),
					LastFailedDeliveryReason: "",
				}
				return sub
			}(),
			wantResult: false,
			wantError:  nil,
		},
	}

	logger, err := eventinglogger.New(string(kymalogger.JSON), string(kymalogger.INFO))
	if err != nil {
		t.Fatalf(`failed to initiate logger, %v`, err)
	}

	r := Reconciler{
		logger: logger,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := r.checkLastFailedDelivery(tc.givenSubscription)
			assert.Equal(t, tc.wantResult, result)
			if tc.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, tc.wantError, err)
			}
		})
	}
}

// helper functions and structs

// testEnvironment provides mocked resources for tests.
type testEnvironment struct {
	fakeClient  client.Client
	backend     *mocks.Backend
	logger      *eventinglogger.Logger
	recorder    *record.FakeRecorder
	cfg         env.Config
	credentials *beb.OAuth2ClientCredentials
	mapper      backendutils.NameMapper
	cleaner     cleaner.Cleaner
}

// setupTestEnvironment is a testEnvironment constructor.
func setupTestEnvironment(t *testing.T, objs ...client.Object) *testEnvironment {
	mockedBackend := &mocks.Backend{}
	fakeClient := createFakeClientBuilder(t).WithObjects(objs...).Build()
	recorder := &record.FakeRecorder{}
	defaultLogger, err := eventinglogger.New(string(kymalogger.JSON), string(kymalogger.INFO))
	if err != nil {
		t.Fatalf("initialize logger failed: %v", err)
	}
	emptyConfig := env.Config{}
	credentials := &beb.OAuth2ClientCredentials{}
	nameMapper := backendutils.NewBEBSubscriptionNameMapper(domain, beb.MaxBEBSubscriptionNameLength)
	cleaner := cleaner.NewEventMeshCleaner(nil)

	return &testEnvironment{
		fakeClient:  fakeClient,
		backend:     mockedBackend,
		logger:      defaultLogger,
		recorder:    recorder,
		cfg:         emptyConfig,
		credentials: credentials,
		mapper:      nameMapper,
		cleaner:     cleaner,
	}
}

func createFakeClientBuilder(t *testing.T) *fake.ClientBuilder {
	err := eventingv1alpha2.AddToScheme(scheme.Scheme)
	require.NoError(t, err)
	err = apigatewayv1beta1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)
	return fake.NewClientBuilder().WithScheme(scheme.Scheme)
}
