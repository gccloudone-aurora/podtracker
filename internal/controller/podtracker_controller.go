/*

MIT License

Copyright (c) His Majesty the King in Right of Canada, as represented by the
Minister responsible for Shared Services Canada, 2024

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/gccloudone-aurora/podtracker/api/v1"
	"github.com/gccloudone-aurora/podtracker/internal/config"
	"github.com/gccloudone-aurora/podtracker/internal/finalizer"
)

// PodTrackerReconciler reconciles a PodTracker object
type PodTrackerReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	PodTrackerConfig *config.CachedPodTrackerConfig
}

//+kubebuilder:rbac:groups=networking.ssc-spc.gc.ca,resources=podtrackers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.ssc-spc.gc.ca,resources=podtrackers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.ssc-spc.gc.ca,resources=podtrackers/finalizers,verbs=update

func removePodTrackerAtIndex(podTrackerItems []v1.PodTracker, index int) []v1.PodTracker {
	podTrackerItems[index] = podTrackerItems[len(podTrackerItems)-1]
	return podTrackerItems[:len(podTrackerItems)-1]
}

// Reconcile is used to reconcile the desired state of PodTracker CRs
func (r *PodTrackerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rl := log.FromContext(ctx)

	// aquire the cached config
	r.PodTrackerConfig.Lock()
	defer r.PodTrackerConfig.Unlock()

	podTracker := &v1.PodTracker{}
	if err := r.Client.Get(ctx, req.NamespacedName, podTracker); err != nil {
		if apierrors.IsNotFound(err) {
			rl.V(2).Info(
				"Request object not found for PodTracker, could have been deleted after reconcile request.",
				"name", req.Name,
			)

			// return and don't requeue
			return ctrl.Result{}, nil
		}

		// error getting PodTracker from API server - return and requeue
		return ctrl.Result{}, err
	}

	if contains, index := r.PodTrackerConfig.Contains(podTracker); contains {
		if !podTracker.ObjectMeta.DeletionTimestamp.IsZero() {
			rl.V(2).Info(
				"PodTracker is queued for deletion",
				"name", podTracker.GetName(),
			)

			controllerutil.RemoveFinalizer(podTracker, finalizer.POD_TRACKER_FINALIZER_NAME)
			if err := r.Update(ctx, podTracker); err != nil {
				if apierrors.IsNotFound(err) {
					// PodTracker may already be gone - reutrn and don't requeue the request
					return ctrl.Result{}, nil
				}

				// error removing finalizer - return and requeue
				return ctrl.Result{}, err
			}

			// remove the PodTracker configuration from the in-memory store
			r.PodTrackerConfig.Items = removePodTrackerAtIndex(r.PodTrackerConfig.Items, index)
			rl.Info(
				"PodTracker resource has been deleted",
				"name", podTracker.GetName(),
			)

			// successfully handled PodTracker deletion - return and don't requeue
			return ctrl.Result{}, nil
		}

		// update the in-memory store with the new PodTracker configuration
		r.PodTrackerConfig.Items[index] = *podTracker
		rl.Info(
			"PodTracker resource has been updated",
			"name", podTracker.GetName(),
			"watchedNamespaces", podTracker.Spec.NSToWatch,
		)

		// in-memory PodTracker configuration has been updated successfully - return and don't requeue
		return ctrl.Result{}, nil
	}

	// add a finalizer to the PodTracker so that we can update the in-memory store prior to deletion
	if !controllerutil.ContainsFinalizer(podTracker, finalizer.POD_TRACKER_FINALIZER_NAME) {
		controllerutil.AddFinalizer(podTracker, finalizer.POD_TRACKER_FINALIZER_NAME)
		if err := r.Update(ctx, podTracker); err != nil {
			if apierrors.IsNotFound(err) {
				// PodTracker was likely deleted before reconciliation - return and don't requeue
				return ctrl.Result{}, nil
			}

			// error updating the Pod - return and requeue
			return ctrl.Result{}, err
		}
	}

	// add the new PodTracker configuration to the in-memory store
	r.PodTrackerConfig.Items = append(r.PodTrackerConfig.Items, *podTracker)
	rl.Info(
		"New PodTracker resource has been registered",
		"name", podTracker.GetName(),
		"watchedNamespaces", podTracker.Spec.NSToWatch,
	)

	// new PodTracker has been added to the in-memory configuration successfully - return and don't requeue
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *PodTrackerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("PodTracker").
		For(
			&v1.PodTracker{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}
