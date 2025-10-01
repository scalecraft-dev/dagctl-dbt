package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/robfig/cron/v3"
	orchestrationv1alpha1 "github.com/scalecraft/dagctl-dbt/api/v1alpha1"
)

type DbtProjectReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Scheduler *cron.Cron
}

// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtprojects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtprojects/finalizers,verbs=update
// +kubebuilder:rbac:groups=orchestration.scalecraft.io,resources=dbtruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets,verbs=get;list;watch

func (r *DbtProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var dbtProject orchestrationv1alpha1.DbtProject
	if err := r.Get(ctx, req.NamespacedName, &dbtProject); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if dbtProject.Status.Phase == "" {
		dbtProject.Status.Phase = orchestrationv1alpha1.DbtProjectPhaseReady
		dbtProject.Status.ObservedGeneration = dbtProject.Generation
		if err := r.Status().Update(ctx, &dbtProject); err != nil {
			return ctrl.Result{}, err
		}
	}

	if dbtProject.Spec.Suspend {
		if dbtProject.Status.Phase != orchestrationv1alpha1.DbtProjectPhaseSuspended {
			dbtProject.Status.Phase = orchestrationv1alpha1.DbtProjectPhaseSuspended
			if err := r.Status().Update(ctx, &dbtProject); err != nil {
				return ctrl.Result{}, err
			}
		}
		r.removeScheduledJob(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if dbtProject.Spec.Schedule != "" {
		if err := r.scheduleProject(ctx, &dbtProject); err != nil {
			log.Error(err, "Failed to schedule project")
			dbtProject.Status.Phase = orchestrationv1alpha1.DbtProjectPhaseError
			r.Status().Update(ctx, &dbtProject)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	if dbtProject.Status.Phase != orchestrationv1alpha1.DbtProjectPhaseReady {
		dbtProject.Status.Phase = orchestrationv1alpha1.DbtProjectPhaseReady
		if err := r.Status().Update(ctx, &dbtProject); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DbtProjectReconciler) scheduleProject(ctx context.Context, project *orchestrationv1alpha1.DbtProject) error {
	jobID := fmt.Sprintf("%s/%s", project.Namespace, project.Name)

	r.removeScheduledJob(types.NamespacedName{
		Namespace: project.Namespace,
		Name:      project.Name,
	})

	entryID, err := r.Scheduler.AddFunc(project.Spec.Schedule, func() {
		r.createScheduledRun(project)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	scheduledJobs[jobID] = entryID
	return nil
}

var scheduledJobs = make(map[string]cron.EntryID)

func (r *DbtProjectReconciler) removeScheduledJob(key types.NamespacedName) {
	jobID := fmt.Sprintf("%s/%s", key.Namespace, key.Name)
	if entryID, exists := scheduledJobs[jobID]; exists {
		r.Scheduler.Remove(entryID)
		delete(scheduledJobs, jobID)
	}
}

func (r *DbtProjectReconciler) createScheduledRun(project *orchestrationv1alpha1.DbtProject) {
	ctx := context.Background()
	log := log.FromContext(ctx)

	run := &orchestrationv1alpha1.DbtRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", project.Name),
			Namespace:    project.Namespace,
		},
		Spec: orchestrationv1alpha1.DbtRunSpec{
			ProjectRef: corev1.LocalObjectReference{
				Name: project.Name,
			},
			Type:     orchestrationv1alpha1.RunTypeScheduled,
			Commands: project.Spec.Commands,
		},
	}

	if err := controllerutil.SetControllerReference(project, run, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference")
		return
	}

	if err := r.Create(ctx, run); err != nil {
		log.Error(err, "Failed to create scheduled run")
		return
	}

	now := metav1.Now()
	project.Status.LastScheduledTime = &now
	if err := r.Status().Update(ctx, project); err != nil {
		log.Error(err, "Failed to update project status")
	}
}

func (r *DbtProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Scheduler == nil {
		r.Scheduler = cron.New(cron.WithSeconds())
		r.Scheduler.Start()
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&orchestrationv1alpha1.DbtProject{}).
		Owns(&orchestrationv1alpha1.DbtRun{}).
		Watches(
			&orchestrationv1alpha1.DbtRun{},
			handler.EnqueueRequestsFromMapFunc(r.findProjectForRun),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *DbtProjectReconciler) findProjectForRun(ctx context.Context, obj client.Object) []reconcile.Request {
	run, ok := obj.(*orchestrationv1alpha1.DbtRun)
	if !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      run.Spec.ProjectRef.Name,
				Namespace: run.Namespace,
			},
		},
	}
}
