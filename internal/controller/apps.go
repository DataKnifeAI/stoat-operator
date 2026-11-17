package controller

import (
	"context"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func (r *StoatReconciler) reconcileApps(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	images := resolveImages(instance.Spec)

	type app struct {
		name      string
		component string
		image     string
		port      int32
	}
	apps := []app{
		{svcAPI, "api", images["api"], portAPI},
		{svcEvents, "events", images["events"], portEvents},
		{svcAutumn, "autumn", images["autumn"], portAutumn},
		{svcJanuary, "january", images["january"], portJanuary},
		{svcGifbox, "gifbox", images["gifbox"], portGifbox},
		{svcCrond, "crond", images["crond"], 0},
		{svcPushd, "pushd", images["pushd"], 0},
	}
	if liveKitEnabled(instance.Spec) {
		apps = append(apps, app{svcVoiceIngress, "voice-ingress", images["voice-ingress"], portVoiceIngress})
	}

	for _, a := range apps {
		ports := []corev1.ContainerPort{}
		svcPorts := []corev1.ServicePort{}
		var readiness, liveness *corev1.Probe
		if a.port > 0 {
			ports = []corev1.ContainerPort{containerPort("http", a.port)}
			svcPorts = []corev1.ServicePort{servicePort("http", a.port)}
			readiness = tcpProbe(a.port)
			liveness = tcpProbe(a.port)
		}
		if err := r.reconcileDeployment(ctx, instance, deploySpec{
			Name:         a.name,
			Component:    a.component,
			Image:        a.image,
			Ports:        ports,
			EnvFrom:      secretEnvFrom(instance),
			VolumeMounts: revoltVolumeMounts(),
			Volumes:      revoltVolumes(),
			Readiness:    readiness,
			Liveness:     liveness,
			Resources:    defaultCPUMem("50m", "128Mi"),
		}); err != nil {
			return ensureErr(a.component+" deployment", err)
		}
		if len(svcPorts) > 0 {
			if err := r.reconcileService(ctx, instance, a.name, a.component, svcPorts, corev1.ServiceTypeClusterIP); err != nil {
				return ensureErr(a.component+" service", err)
			}
		}
	}

	// Web front-end (env from ConfigMap; no Revolt.toml mount required).
	if err := r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcWeb,
		Component: "web",
		Image:     images["web"],
		Ports:     []corev1.ContainerPort{containerPort("http", portWeb)},
		EnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: webEnvName},
			},
		}},
		Readiness: tcpProbe(portWeb),
		Liveness:  tcpProbe(portWeb),
		Resources: defaultCPUMem("50m", "128Mi"),
	}); err != nil {
		return ensureErr("web deployment", err)
	}
	if err := r.reconcileService(ctx, instance, svcWeb, "web",
		[]corev1.ServicePort{servicePort("http", portWeb)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("web service", err)
	}

	return nil
}
