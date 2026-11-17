package controller

import (
	"context"

	stoatv1alpha1 "github.com/DataKnifeAI/stoat-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *StoatReconciler) reconcileDependencies(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.MongoDB == nil {
		if err := r.reconcileMongo(ctx, instance); err != nil {
			return err
		}
	}
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.Redis == nil {
		if err := r.reconcileRedis(ctx, instance); err != nil {
			return err
		}
	}
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.RabbitMQ == nil {
		if err := r.reconcileRabbit(ctx, instance); err != nil {
			return err
		}
	}
	if boolDefault(instance.Spec.Dependencies.Embed, true) && instance.Spec.Dependencies.ObjectStorage == nil {
		if err := r.reconcileMinio(ctx, instance); err != nil {
			return err
		}
		if err := r.reconcileCreateBucketsJob(ctx, instance); err != nil {
			return err
		}
	}
	return nil
}

func (r *StoatReconciler) reconcileMongo(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if err := r.reconcilePVC(ctx, instance, "stoat-mongo-data", "database"); err != nil {
		return ensureErr("mongo pvc", err)
	}
	if err := r.reconcileService(ctx, instance, svcDatabase, "database",
		[]corev1.ServicePort{servicePort("mongo", portMongo)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("mongo service", err)
	}
	return ensureErr("mongo deployment", r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcDatabase,
		Component: "database",
		Image:     mongoImage,
		Ports:     []corev1.ContainerPort{containerPort("mongo", portMongo)},
		VolumeMounts: []corev1.VolumeMount{{
			Name: "data", MountPath: "/data/db",
		}},
		Volumes:   []corev1.Volume{pvcVolume("data", "stoat-mongo-data")},
		Readiness: tcpProbe(portMongo),
		Liveness:  tcpProbe(portMongo),
		Resources: defaultCPUMem("100m", "512Mi"),
		PodSecurityContext: &corev1.PodSecurityContext{
			FSGroup: int64Ptr(999),
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:    int64Ptr(999),
			RunAsNonRoot: boolPtr(true),
		},
	}))
}

func (r *StoatReconciler) reconcileRedis(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if err := r.reconcileService(ctx, instance, svcRedis, "redis",
		[]corev1.ServicePort{servicePort("redis", portRedis)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("redis service", err)
	}
	return ensureErr("redis deployment", r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcRedis,
		Component: "redis",
		Image:     redisImage,
		Ports:     []corev1.ContainerPort{containerPort("redis", portRedis)},
		Readiness: tcpProbe(portRedis),
		Liveness:  tcpProbe(portRedis),
		Resources: defaultCPUMem("50m", "128Mi"),
	}))
}

func (r *StoatReconciler) reconcileRabbit(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if err := r.reconcilePVC(ctx, instance, "stoat-rabbit-data", "rabbit"); err != nil {
		return ensureErr("rabbit pvc", err)
	}
	if err := r.reconcileService(ctx, instance, svcRabbit, "rabbit",
		[]corev1.ServicePort{servicePort("amqp", portRabbit)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("rabbit service", err)
	}
	return ensureErr("rabbit deployment", r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcRabbit,
		Component: "rabbit",
		Image:     rabbitImage,
		Ports:     []corev1.ContainerPort{containerPort("amqp", portRabbit)},
		Env: []corev1.EnvVar{
			{Name: "RABBITMQ_DEFAULT_USER", Value: rabbitUser},
			{Name: "RABBITMQ_DEFAULT_PASS", Value: rabbitPass},
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/rabbitmq"}},
		Volumes:      []corev1.Volume{pvcVolume("data", "stoat-rabbit-data")},
		Readiness:    tcpProbe(portRabbit),
		Liveness:     tcpProbe(portRabbit),
		Resources:    defaultCPUMem("100m", "256Mi"),
		PodSecurityContext: &corev1.PodSecurityContext{
			FSGroup: int64Ptr(999),
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:    int64Ptr(999),
			RunAsNonRoot: boolPtr(true),
		},
	}))
}

func (r *StoatReconciler) reconcileMinio(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	if err := r.reconcilePVC(ctx, instance, "stoat-minio-data", "minio"); err != nil {
		return ensureErr("minio pvc", err)
	}
	if err := r.reconcileService(ctx, instance, svcMinio, "minio",
		[]corev1.ServicePort{servicePort("s3", portMinio)}, corev1.ServiceTypeClusterIP); err != nil {
		return ensureErr("minio service", err)
	}
	return ensureErr("minio deployment", r.reconcileDeployment(ctx, instance, deploySpec{
		Name:      svcMinio,
		Component: "minio",
		Image:     minioImage,
		// Keep image ENTRYPOINT; pass args like compose `command: server /data`.
		Args:  []string{"server", "/data"},
		Ports: []corev1.ContainerPort{containerPort("s3", portMinio)},
		Env: []corev1.EnvVar{
			{Name: "MINIO_ROOT_USER", Value: minioUser},
			{Name: "MINIO_ROOT_PASSWORD", Value: minioPass},
			{Name: "MINIO_DOMAIN", Value: svcMinio},
		},
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
		Volumes:      []corev1.Volume{pvcVolume("data", "stoat-minio-data")},
		Readiness:    tcpProbe(portMinio),
		Liveness:     tcpProbe(portMinio),
		Resources:    defaultCPUMem("100m", "256Mi"),
	}))
}

func (r *StoatReconciler) reconcileCreateBucketsJob(ctx context.Context, instance *stoatv1alpha1.Stoat) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stoat-createbuckets",
			Namespace: instance.Namespace,
		},
	}
	// Jobs are largely immutable; create if missing, otherwise leave alone.
	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: instance.Namespace}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	backoff := int32(10)
	ttl := int32(600)
	job.Labels = componentLabels(instance.Name, "createbuckets")
	job.Spec.BackoffLimit = &backoff
	job.Spec.TTLSecondsAfterFinished = &ttl
	job.Spec.Template.ObjectMeta.Labels = componentLabels(instance.Name, "createbuckets")
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	job.Spec.Template.Spec.Containers = []corev1.Container{{
		Name:    "mc",
		Image:   mcImage,
		Command: []string{"/bin/sh", "-c"},
		Args: []string{`
set -e
while ! /usr/bin/mc alias set minio http://minio:9000 minioautumn minioautumn; do
  echo 'Waiting for minio...'
  sleep 2
done
while ! /usr/bin/mc ready minio; do
  echo 'Minio not ready...'
  sleep 2
done
/usr/bin/mc mb --ignore-existing minio/revolt-uploads
`},
	}}
	if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, job)
}
