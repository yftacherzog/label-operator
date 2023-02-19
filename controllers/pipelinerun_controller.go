/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	addRunNameLabelAnnotation = "yherzog.il/add-run-name-label"
	runNameLabel              = "yherzog.il/run-name"
	finalizerName             = "yherzog.il/my-finalizer"
)

// PipelineRunReconciler reconciles a PipelineRun object
type PipelineRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tekton.yherzog.il,resources=pipelineruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tekton.yherzog.il,resources=pipelineruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tekton.yherzog.il,resources=pipelineruns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PipelineRun object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PipelineRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile start...")

	// get the pipelinerun
	pipelineRun := &tektonv1beta1.PipelineRun{}
	err := r.Get(ctx, req.NamespacedName, pipelineRun)
	if err != nil {
		logger.Error(err, "Failed to get pipelineRun for", "req", req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// add or remove the label
	labelShouldBePresent := pipelineRun.Annotations[addRunNameLabelAnnotation] == "true"
	labelIsPresent := pipelineRun.Labels[runNameLabel] == pipelineRun.Name

	if pipelineRun.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pipelineRun, finalizerName) {
			logger.Info("Adding finalizer")
			controllerutil.AddFinalizer(pipelineRun, finalizerName)
			if err := r.Update(ctx, pipelineRun); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(pipelineRun, finalizerName) {
			logger.Info("Deleting finalizer")
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(pipelineRun, finalizerName)
			if err := r.Update(ctx, pipelineRun); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if labelShouldBePresent == labelIsPresent {
		// The desired state and actual state of the Run are the same.
		// No further action is required by the operator at this moment.
		logger.Info("no update required")
		return ctrl.Result{}, nil
	}

	if labelShouldBePresent {
		// If the label should be set but is not, set it.
		if pipelineRun.Labels == nil {
			pipelineRun.Labels = make(map[string]string)
		}
		pipelineRun.Labels[runNameLabel] = pipelineRun.Name
		logger.Info("adding label")
	} else {
		// If the label should not be set but is, remove it.
		// TODO: try printing the existing label.
		delete(pipelineRun.Labels, runNameLabel)
		logger.Info("removing label")
	}

	// push the pipelinerun to the kubernetes api
	if err := r.Update(ctx, pipelineRun); err != nil {
		if errors.IsConflict(err) {
			// The Run has been updated since we read it.
			// Requeue the Run to try to reconciliate again.
			logger.Info("requeuing due to conflict")
			return ctrl.Result{Requeue: true}, nil
		}
		if errors.IsNotFound(err) {
			// The Run has been deleted since we read it.
			// Requeue the Run to try to reconciliate again.
			logger.Info("requeuing due to deletion")
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "unable to update Run")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tektonv1beta1.PipelineRun{}).
		Complete(r)
}
