package controller

import (
	"context"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	defaultGatewayName      = "traefik-gateway"
	defaultGatewayNamespace = "kube-system"
	defaultListenerName     = "websecure"
)

func gatewayName(spec stoatv1alpha1.StoatSpec) string {
	if n := strings.TrimSpace(spec.Ingress.GatewayName); n != "" {
		return n
	}
	return defaultGatewayName
}

func gatewayNamespace(spec stoatv1alpha1.StoatSpec) string {
	if n := strings.TrimSpace(spec.Ingress.GatewayNamespace); n != "" {
		return n
	}
	return defaultGatewayNamespace
}

func listenerName(spec stoatv1alpha1.StoatSpec) string {
	if n := strings.TrimSpace(spec.Ingress.ListenerName); n != "" {
		return n
	}
	return defaultListenerName
}

func (r *StoatReconciler) usesGatewayClass(ctx context.Context, className string) bool {
	gc := &gatewayv1.GatewayClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: className}, gc); err != nil {
		return false
	}
	return true
}

func (r *StoatReconciler) deleteIngressIfExists(ctx context.Context, namespace, name string) error {
	ing := &networkingv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, ing)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.Delete(ctx, ing)
}

func (r *StoatReconciler) deleteHTTPRouteIfExists(ctx context.Context, namespace, name string) error {
	route := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, route)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return r.Delete(ctx, route)
}

func (r *StoatReconciler) reconcileHTTPRoute(ctx context.Context, instance *stoatv1alpha1.Stoat, name, host, svcName string, port int32, labels map[string]string) error {
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	pathPrefix := gatewayv1.PathMatchPathPrefix
	pathValue := "/"
	gwNs := gatewayv1.Namespace(gatewayNamespace(instance.Spec))
	section := gatewayv1.SectionName(listenerName(instance.Spec))
	portNum := gatewayv1.PortNumber(port)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		if err := controllerutil.SetControllerReference(instance, route, r.Scheme); err != nil {
			return err
		}
		route.Labels = labels
		annotations := map[string]string{}
		for k, v := range instance.Spec.Ingress.Annotations {
			annotations[k] = v
		}
		route.Annotations = annotations

		route.Spec.ParentRefs = []gatewayv1.ParentReference{{
			Name:        gatewayv1.ObjectName(gatewayName(instance.Spec)),
			Namespace:   &gwNs,
			SectionName: &section,
		}}
		route.Spec.Hostnames = []gatewayv1.Hostname{gatewayv1.Hostname(host)}
		route.Spec.Rules = []gatewayv1.HTTPRouteRule{{
			Matches: []gatewayv1.HTTPRouteMatch{{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  &pathPrefix,
					Value: &pathValue,
				},
			}},
			BackendRefs: []gatewayv1.HTTPBackendRef{{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: gatewayv1.ObjectName(svcName),
						Port: &portNum,
					},
				},
			}},
		}}
		return nil
	})
	return err
}
