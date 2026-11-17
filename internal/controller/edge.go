package controller

import (
	"context"
	"strconv"
	"strings"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *StoatReconciler) reconcileEdge(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	images := resolveImages(instance.Spec)

	if err := r.reconcileCaddy(ctx, instance); err != nil {
		return err
	}
	if err := r.reconcileIngress(ctx, instance); err != nil {
		return err
	}
	if liveKitEnabled(instance.Spec) {
		if err := r.reconcileLiveKit(ctx, instance, images["livekit"]); err != nil {
			return err
		}
	}
	return nil
}

func (r *StoatReconciler) reconcileCaddy(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if err := r.reconcileService(ctx, instance, svcCaddy, "caddy",
		[]corev1.ServicePort{servicePort("http", portCaddy)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("caddy service", err)
	}
	return ensureErr("caddy deployment", r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcCaddy,
		Component: "caddy",
		Image:     caddyImage,
		Ports:     []corev1.ContainerPort{containerPort("http", portCaddy)},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "caddyfile", MountPath: "/etc/caddy/Caddyfile", SubPath: "Caddyfile", ReadOnly: true},
			{Name: "config", MountPath: "/config", ReadOnly: true},
			{Name: "data", MountPath: "/data"},
		},
		Volumes: []corev1.Volume{
			{
				Name: "caddyfile",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: caddyCfgName},
						Items:                []corev1.KeyToPath{{Key: "Caddyfile", Path: "Caddyfile"}},
					},
				},
			},
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configName},
						Items:                []corev1.KeyToPath{{Key: "stoat.json", Path: "stoat.json"}},
					},
				},
			},
			emptyDirVolume("data"),
		},
		Readiness: tcpProbe(portCaddy),
		Liveness:  tcpProbe(portCaddy),
		Resources: defaultCPUMem("50m", "64Mi"),
	}))
}

func (r *StoatReconciler) reconcileIngress(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stoat",
			Namespace: instance.Namespace,
		},
	}
	pathType := networkingv1.PathTypePrefix
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		if err := controllerutil.SetControllerReference(instance, ing, r.Scheme); err != nil {
			return err
		}
		ing.Labels = stoatLabels(instance.Name)
		annotations := map[string]string{}
		for k, v := range instance.Spec.Ingress.Annotations {
			annotations[k] = v
		}
		ing.Annotations = annotations

		className := ingressClass(instance.Spec)
		ing.Spec.IngressClassName = &className
		ing.Spec.Rules = []networkingv1.IngressRule{{
			Host: instance.Spec.Domain,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path:     "/",
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: svcCaddy,
								Port: networkingv1.ServiceBackendPort{Number: portCaddy},
							},
						},
					}},
				},
			},
		}}
		if instance.Spec.Ingress.TLSSecretName != "" {
			ing.Spec.TLS = []networkingv1.IngressTLS{{
				Hosts:      []string{instance.Spec.Domain},
				SecretName: instance.Spec.Ingress.TLSSecretName,
			}}
		} else {
			ing.Spec.TLS = nil
		}
		return nil
	})
	return ensureErr("ingress", err)
}

func (r *StoatReconciler) reconcileLiveKit(ctx context.Context, instance *stoatv1alpha1.Stoat, image string) error {
	rtc, udpStart, udpEnd := liveKitPorts(instance.Spec)
	labels := componentLabels(instance.Name, "livekit")
	replicas := int32(1)

	// ClusterIP for in-cluster HTTP signaling (Caddy /livekit*).
	if err := r.reconcileService(ctx, instance, svcLiveKit, "livekit",
		[]corev1.ServicePort{
			servicePort("http", portLiveKitHTTP),
			servicePort("rtc", rtc),
		}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("livekit service", err)
	}

	if err := r.reconcileLiveKitRTCService(ctx, instance, rtc, udpStart, udpEnd); err != nil {
		return err
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcLiveKit,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := controllerutil.SetControllerReference(instance, deploy, r.Scheme); err != nil {
			return err
		}
		deploy.Labels = labels
		deploy.Spec.Replicas = &replicas
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deploy.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
		deploy.Spec.Template.ObjectMeta.Labels = labels
		deploy.Spec.Template.Spec.HostNetwork = true
		deploy.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
		// Service links inject LIVEKIT_PORT=tcp://... which LiveKit misparses as its listen port.
		deploy.Spec.Template.Spec.EnableServiceLinks = boolPtr(false)
		deploy.Spec.Template.Spec.ImagePullSecrets = instance.Spec.ImagePullSecrets
		deploy.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
		deploy.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:            "livekit",
			Image:           image,
			ImagePullPolicy: pullPolicy(instance.Spec),
			Args:            []string{"--config", "/etc/livekit.yml"},
			Env:            []corev1.EnvVar{liveKitNodeIPEnv(instance.Spec)},
			Ports: []corev1.ContainerPort{
				containerPort("http", portLiveKitHTTP),
				containerPort("rtc", rtc),
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name: "livekit", MountPath: "/etc/livekit.yml", SubPath: "livekit.yml", ReadOnly: true,
			}},
			ReadinessProbe: tcpProbe(portLiveKitHTTP),
			LivenessProbe:  tcpProbe(portLiveKitHTTP),
			Resources:      defaultCPUMem("100m", "256Mi"),
		}}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "livekit",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: livekitCfgName},
					Items:                []corev1.KeyToPath{{Key: "livekit.yml", Path: "livekit.yml"}},
				},
			},
		}}
		return nil
	})
	return ensureErr("livekit deployment", err)
}

func liveKitNodeIPEnv(spec stoatv1alpha1.StoatSpec) corev1.EnvVar {
	if addr := strings.TrimSpace(spec.LiveKit.Address); addr != "" {
		return corev1.EnvVar{Name: "NODE_IP", Value: addr}
	}
	return corev1.EnvVar{
		Name: "NODE_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
		},
	}
}

func (r *StoatReconciler) reconcileLiveKitRTCService(ctx context.Context, instance *stoatv1alpha1.Stoat, rtc, udpStart, udpEnd int32) error {
	svcType := instance.Spec.LiveKit.ServiceType
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}
	addr := strings.TrimSpace(instance.Spec.LiveKit.Address)

	// Only claim a VIP Service when explicitly asking for LoadBalancer + address.
	if svcType != corev1.ServiceTypeLoadBalancer || addr == "" {
		return nil
	}

	ports := []corev1.ServicePort{servicePort("rtc-tcp", rtc)}
	for p := udpStart; p <= udpEnd; p++ {
		ports = append(ports, servicePortUDP(
			"rtc-udp-"+strconv.FormatInt(int64(p), 10), p,
		))
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcLiveKitRTC,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(instance, svc, r.Scheme); err != nil {
			return err
		}
		svc.Labels = componentLabels(instance.Name, "livekit-rtc")
		svc.Spec.Selector = componentLabels(instance.Name, "livekit")
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		svc.Spec.LoadBalancerIP = addr
		svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
		svc.Spec.Ports = ports
		return nil
	})
	return ensureErr("livekit-rtc service", err)
}
