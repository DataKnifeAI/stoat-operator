package controller

import (
	"context"
	"fmt"
	"time"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// StoatReconciler reconciles a Stoat object into a self-hosted stack:
// secrets/config, optional embedded dependencies, app Deployments, Caddy edge,
// Ingress, and LiveKit (hostNetwork for RTC).
type StoatReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=stoat.dataknife.ai,resources=stoats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stoat.dataknife.ai,resources=stoats/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=stoat.dataknife.ai,resources=stoats/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch

func (r *StoatReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	instance := &stoatv1alpha1.Stoat{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(instance, finalizerName) {
		controllerutil.AddFinalizer(instance, finalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if !instance.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(instance, finalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if instance.Spec.Domain == "" {
		return r.failStatus(ctx, instance, "Domain is required")
	}

	secrets, err := r.reconcileSecrets(ctx, instance)
	if err != nil {
		return r.failStatus(ctx, instance, fmt.Sprintf("Secrets: %v", err))
	}
	if err := r.reconcileConfig(ctx, instance, secrets); err != nil {
		return r.failStatus(ctx, instance, fmt.Sprintf("Config: %v", err))
	}
	if err := r.reconcileDependencies(ctx, instance); err != nil {
		return r.failStatus(ctx, instance, fmt.Sprintf("Dependencies: %v", err))
	}
	if err := r.reconcileApps(ctx, instance); err != nil {
		return r.failStatus(ctx, instance, fmt.Sprintf("Apps: %v", err))
	}
	if err := r.reconcileEdge(ctx, instance); err != nil {
		return r.failStatus(ctx, instance, fmt.Sprintf("Edge: %v", err))
	}

	ready, message := r.observeReady(ctx, instance)
	version := versionOrDefault(instance.Spec)
	phase := stoatv1alpha1.PhasePending
	if ready {
		phase = stoatv1alpha1.PhaseRunning
	}

	instance.Status.Phase = phase
	instance.Status.Ready = ready
	instance.Status.URL = publicURL(instance.Spec.Domain)
	instance.Status.ObservedVersion = version
	instance.Status.Message = message
	condStatus := metav1.ConditionFalse
	reason := "Pending"
	if ready {
		condStatus = metav1.ConditionTrue
		reason = "Running"
	}
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: instance.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update Stoat status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled Stoat", "domain", instance.Spec.Domain, "version", version, "ready", ready)
	if !ready {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *StoatReconciler) observeReady(ctx context.Context, instance *stoatv1alpha1.Stoat) (bool, string) {
	required := []string{svcAPI, svcEvents, svcWeb, svcCaddy, svcAutumn}
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.MongoDB == nil {
		required = append(required, svcDatabase)
	}
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.Redis == nil {
		required = append(required, svcRedis)
	}
	if liveKitEnabled(instance.Spec) {
		required = append(required, svcLiveKit)
	}

	for _, name := range required {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, deploy); err != nil {
			return false, fmt.Sprintf("Waiting for Deployment %s", name)
		}
		if deploy.Status.ReadyReplicas < 1 {
			return false, fmt.Sprintf("Waiting for Deployment %s to become ready", name)
		}
	}
	return true, "Stoat stack is ready"
}

func (r *StoatReconciler) failStatus(ctx context.Context, instance *stoatv1alpha1.Stoat, message string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	instance.Status.Phase = stoatv1alpha1.PhaseFailed
	instance.Status.Ready = false
	instance.Status.Message = message
	instance.Status.URL = publicURL(instance.Spec.Domain)
	instance.Status.ObservedVersion = versionOrDefault(instance.Spec)
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             stoatv1alpha1.PhaseFailed,
		Message:            message,
		ObservedGeneration: instance.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})
	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update Stoat status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stoatv1alpha1.Stoat{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Named("stoat").
		Complete(r)
}
