package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	orchestrationv1alpha1 "github.com/scalecraft/dagctl-dbt/api/v1alpha1"
)

type DbtRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtruns/finalizers,verbs=update
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtprojects,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets,verbs=get;list;watch

func (r *DbtRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var dbtRun orchestrationv1alpha1.DbtRun
	if err := r.Get(ctx, req.NamespacedName, &dbtRun); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	var project orchestrationv1alpha1.DbtProject
	projectKey := client.ObjectKey{
		Namespace: dbtRun.Namespace,
		Name:      dbtRun.Spec.ProjectRef.Name,
	}
	if err := r.Get(ctx, projectKey, &project); err != nil {
		log.Error(err, "Failed to fetch DbtProject")
		return ctrl.Result{}, err
	}

	if dbtRun.Status.Phase == "" {
		dbtRun.Status.Phase = orchestrationv1alpha1.RunPhasePending
		if err := r.Status().Update(ctx, &dbtRun); err != nil {
			return ctrl.Result{}, err
		}
	}

	if dbtRun.Status.JobRef == nil {
		job, err := r.createJob(ctx, &dbtRun, &project)
		if err != nil {
			log.Error(err, "Failed to create Job")
			dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseError
			r.Status().Update(ctx, &dbtRun)
			return ctrl.Result{}, err
		}

		dbtRun.Status.JobRef = &corev1.ObjectReference{
			Kind:       "Job",
			APIVersion: "batch/v1",
			Name:       job.Name,
			Namespace:  job.Namespace,
			UID:        job.UID,
		}

		now := metav1.Now()
		dbtRun.Status.StartTime = &now
		dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseRunning

		if err := r.Status().Update(ctx, &dbtRun); err != nil {
			return ctrl.Result{}, err
		}
	}

	var job batchv1.Job
	jobKey := client.ObjectKey{
		Namespace: dbtRun.Status.JobRef.Namespace,
		Name:      dbtRun.Status.JobRef.Name,
	}
	if err := r.Get(ctx, jobKey, &job); err != nil {
		if apierrors.IsNotFound(err) {
			dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseError
			r.Status().Update(ctx, &dbtRun)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	dbtRun.Status.JobStatus = job.Status.DeepCopy()

	if job.Status.Succeeded > 0 {
		dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseSucceeded
		now := metav1.Now()
		dbtRun.Status.CompletionTime = &now

		project.Status.LastSuccessfulTime = &now
		if err := r.Status().Update(ctx, &project); err != nil {
			log.Error(err, "Failed to update project status")
		}
	} else if job.Status.Failed > 0 {
		dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseFailed
		now := metav1.Now()
		dbtRun.Status.CompletionTime = &now
	} else {
		dbtRun.Status.Phase = orchestrationv1alpha1.RunPhaseRunning
	}

	if err := r.Status().Update(ctx, &dbtRun); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DbtRunReconciler) createJob(ctx context.Context, run *orchestrationv1alpha1.DbtRun, project *orchestrationv1alpha1.DbtProject) (*batchv1.Job, error) {
	commands := run.Spec.Commands
	if len(commands) == 0 {
		commands = project.Spec.Commands
	}
	if len(commands) == 0 {
		commands = []string{"run"}
	}

	image := project.Spec.Image
	if image == "" {
		image = "ghcr.io/dbt-labs/dbt-postgres:1.7.0"
	}

	initContainers := []corev1.Container{
		{
			Name:  "git-clone",
			Image: "alpine/git:latest",
			Command: []string{
				"sh", "-c",
				fmt.Sprintf("git clone %s /workspace && cd /workspace && git checkout %s",
					project.Spec.Git.Repository,
					getGitRef(project.Spec.Git.Ref)),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
		},
	}

	if project.Spec.Git.SSHKeySecret != "" {
		initContainers[0].VolumeMounts = append(initContainers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "ssh-key",
			MountPath: "/root/.ssh",
		})
	}

	dbtCmd := []string{"dbt"}
	dbtCmd = append(dbtCmd, commands...)

	workDir := "/workspace"
	if project.Spec.Git.Path != "" && project.Spec.Git.Path != "/" {
		workDir = fmt.Sprintf("/workspace/%s", project.Spec.Git.Path)
	}

	container := corev1.Container{
		Name:       "dbt",
		Image:      image,
		Command:    dbtCmd,
		WorkingDir: workDir,
		Env:        project.Spec.Env,
		Resources:  project.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		},
	}

	if project.Spec.ProfilesConfigMap != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "profiles",
			MountPath: "/root/.dbt",
		})
	}

	container.VolumeMounts = append(container.VolumeMounts, project.Spec.VolumeMounts...)

	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if project.Spec.ProfilesConfigMap != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "profiles",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: project.Spec.ProfilesConfigMap,
					},
				},
			},
		})
	}

	if project.Spec.Git.SSHKeySecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ssh-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  project.Spec.Git.SSHKeySecret,
					DefaultMode: ptr.To(int32(0400)),
				},
			},
		})
	}

	// Create labels with run metadata
	labels := map[string]string{
		"app.kubernetes.io/name":               "dagctl-dbt",
		"app.kubernetes.io/component":          "dbt-run",
		"app.kubernetes.io/managed-by":         "dagctl-dbt-operator",
		"orchestration.scalecraft.io/project":  project.Name,
		"orchestration.scalecraft.io/run":      run.Name,
		"orchestration.scalecraft.io/run-type": string(run.Spec.Type),
	}

	// Add command as annotation (labels have character limits)
	annotations := map[string]string{
		"orchestration.scalecraft.io/dbt-command": fmt.Sprintf("dbt %v", commands),
		"orchestration.scalecraft.io/created-at":  metav1.Now().Format("2006-01-02T15:04:05Z"),
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", run.Name, "job"),
			Namespace:   run.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: run.Spec.TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					InitContainers:     initContainers,
					Containers:         []corev1.Container{container},
					Volumes:            volumes,
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: project.Spec.ServiceAccountName,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(run, job, r.Scheme); err != nil {
		return nil, err
	}

	if err := r.Create(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func getGitRef(ref string) string {
	if ref == "" {
		return "main"
	}
	return ref
}

func (r *DbtRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&orchestrationv1alpha1.DbtRun{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
