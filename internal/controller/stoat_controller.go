package controller

import (
	"context"
	"time"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// StoatReconciler reconciles a Stoat object.
// Scaffold: validates the CR, manages the finalizer, and reports Pending status
// until component reconciliation (deps, app Deployments, ingress, LiveKit) is
// implemented in follow-up work.
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
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways;httproutes,verbs=get;list;watch;create;update;patch;delete

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

	_ = resolveImages(instance.Spec)
	_ = stoatLabels(instance.Name)

	version := versionOrDefault(instance.Spec)
	message := "Scaffold ready; component reconciliation not yet implemented"
	instance.Status.Phase = stoatv1alpha1.PhasePending
	instance.Status.Ready = false
	instance.Status.URL = publicURL(instance.Spec.Domain)
	instance.Status.ObservedVersion = version
	instance.Status.Message = message
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "Scaffold",
		Message:            message,
		ObservedGeneration: instance.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})

	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update Stoat status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled Stoat scaffold", "domain", instance.Spec.Domain, "version", version)
	return ctrl.Result{}, nil
}

func (r *StoatReconciler) failStatus(ctx context.Context, instance *stoatv1alpha1.Stoat, message string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	instance.Status.Phase = stoatv1alpha1.PhaseFailed
	instance.Status.Ready = false
	instance.Status.Message = message
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
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StoatReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stoatv1alpha1.Stoat{}).
		Named("stoat").
		Complete(r)
}
