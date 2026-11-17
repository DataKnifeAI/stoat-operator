package controller

import (
	"context"
	"fmt"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type deploySpec struct {
	Name               string
	Component          string
	Image              string
	Replicas           int32
	Command            []string
	Args               []string
	Ports              []corev1.ContainerPort
	Env                []corev1.EnvVar
	EnvFrom            []corev1.EnvFromSource
	VolumeMounts       []corev1.VolumeMount
	Volumes            []corev1.Volume
	Readiness          *corev1.Probe
	Liveness           *corev1.Probe
	Resources          corev1.ResourceRequirements
	SecurityContext    *corev1.SecurityContext
	PodSecurityContext *corev1.PodSecurityContext
}

func (r *StoatReconciler) reconcilePVC(ctx context.Context, instance *stoatv1alpha1.Stoat, name, component string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if err := controllerutil.SetControllerReference(instance, pvc, r.Scheme); err != nil {
			return err
		}
		pvc.Labels = componentLabels(instance.Name, component)
		if pvc.Spec.AccessModes == nil {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		}
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}
		if _, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !ok {
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = storageSizeOrDefault(instance.Spec)
		}
		if instance.Spec.Storage.StorageClassName != "" && pvc.Spec.StorageClassName == nil {
			sc := instance.Spec.Storage.StorageClassName
			pvc.Spec.StorageClassName = &sc
		}
		return nil
	})
	return err
}

func (r *StoatReconciler) reconcileService(ctx context.Context, instance *stoatv1alpha1.Stoat, name, component string, ports []corev1.ServicePort, svcType corev1.ServiceType) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(instance, svc, r.Scheme); err != nil {
			return err
		}
		svc.Labels = componentLabels(instance.Name, component)
		svc.Spec.Selector = componentLabels(instance.Name, component)
		svc.Spec.Type = svcType
		svc.Spec.Ports = ports
		return nil
	})
	return err
}

func (r *StoatReconciler) reconcileDeployment(ctx context.Context, instance *stoatv1alpha1.Stoat, spec deploySpec) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: instance.Namespace,
		},
	}
	labels := componentLabels(instance.Name, spec.Component)
	replicas := spec.Replicas
	if replicas == 0 {
		replicas = 1
	}
	resources := spec.Resources
	if len(instance.Spec.Resources.Requests) > 0 || len(instance.Spec.Resources.Limits) > 0 {
		if resources.Requests == nil && resources.Limits == nil {
			resources = instance.Spec.Resources
		}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := controllerutil.SetControllerReference(instance, deploy, r.Scheme); err != nil {
			return err
		}
		deploy.Labels = labels
		deploy.Spec.Replicas = &replicas
		deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deploy.Spec.Template.ObjectMeta.Labels = labels
		deploy.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
		container := corev1.Container{
			Name:            spec.Component,
			Image:           spec.Image,
			ImagePullPolicy: pullPolicy(instance.Spec),
			Command:         spec.Command,
			Args:            spec.Args,
			Ports:           spec.Ports,
			Env:             spec.Env,
			EnvFrom:         spec.EnvFrom,
			VolumeMounts:    spec.VolumeMounts,
			ReadinessProbe:  spec.Readiness,
			LivenessProbe:   spec.Liveness,
			Resources:       resources,
			SecurityContext: spec.SecurityContext,
		}
		deploy.Spec.Template.Spec.Containers = []corev1.Container{container}
		deploy.Spec.Template.Spec.Volumes = spec.Volumes
		deploy.Spec.Template.Spec.ImagePullSecrets = instance.Spec.ImagePullSecrets
		deploy.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
		deploy.Spec.Template.Spec.SecurityContext = spec.PodSecurityContext
		return nil
	})
	return err
}

func servicePort(name string, port int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt32(port),
		Protocol:   corev1.ProtocolTCP,
	}
}

func servicePortUDP(name string, port int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Port:       port,
		TargetPort: intstr.FromInt32(port),
		Protocol:   corev1.ProtocolUDP,
	}
}

func containerPort(name string, port int32) corev1.ContainerPort {
	return corev1.ContainerPort{Name: name, ContainerPort: port, Protocol: corev1.ProtocolTCP}
}

func revoltVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{{
		Name:      "config",
		MountPath: "/Revolt.toml",
		SubPath:   "Revolt.toml",
		ReadOnly:  true,
	}}
}

func revoltVolumes() []corev1.Volume {
	return []corev1.Volume{{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configName},
				Items:                []corev1.KeyToPath{{Key: "Revolt.toml", Path: "Revolt.toml"}},
			},
		},
	}}
}

func secretEnvFrom(instance *stoatv1alpha1.Stoat) []corev1.EnvFromSource {
	return []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretNameFor(instance)},
		},
	}}
}

func defaultCPUMem(cpu, mem string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(mem),
		},
	}
}

func pvcVolume(name, claim string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim},
		},
	}
}

func emptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func ensureErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
