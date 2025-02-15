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
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gccloudone-aurora/podtracker/internal/config"
	"github.com/gccloudone-aurora/podtracker/internal/finalizer"
	"github.com/gccloudone-aurora/podtracker/internal/tracking"
	"github.com/gccloudone-aurora/podtracker/internal/writer"
)

// PodReconciler reconciles built-in Pod resources
// The goal of this controller is to help provide information about Pod lifetimes with respect to their network configuration (Pod IP allocation)
// which will assist in network auditing and security.
type PodReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	PodTrackerConfig *config.CachedPodTrackerConfig
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update
// NOTE: the list/watch permissions on the following Nodes RBAC are needed due to the client being passed into this reconciler having caching which means informers are created for any Get requests so that they can take advantage of caching (https://github.com/kubernetes-sigs/controller-runtime/issues/1156)
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is used to log pertinant information about Pod create and delete events using the configured BackendWriters
// This function is called to reconcile the above behaviour whenever a Pod is created or has a deletion timestamp added.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rl := log.FromContext(ctx)

	// get the Pod resource from the Kubernetes API
	currentPod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, currentPod); err != nil {
		if apierrors.IsNotFound(err) {
			rl.V(2).Info(
				"Request object not found for Pod, could have been deleted after reconcile request.",
				"name", req.Name,
				"namespace", req.Namespace,
			)

			// return and don't requeue
			return ctrl.Result{}, nil
		}

		// error getting Pod from Kubernetes API server - requeue the request
		return ctrl.Result{}, err
	}

	// get the Node that the Pod resides on (this is necessary to determine the NodeIP which is useful for troubleshooting Pods using `HostNetworking`)
	currentNode := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      currentPod.Spec.NodeName,
		Namespace: corev1.NamespaceAll,
	}, currentNode); err != nil {
		if apierrors.IsNotFound(err) {
			// pod may not be scheduled to a Node yet - return and requeue with a short delay
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// check if the Pod is scheduled for deletion
	if !currentPod.ObjectMeta.DeletionTimestamp.IsZero() {
		rl.V(2).Info(
			"Pod is queued for deletion",
			"name", currentPod.GetName(),
			"namespace", currentPod.GetNamespace(),
		)

		// remove the finalizer on the pod resource
		controllerutil.RemoveFinalizer(currentPod, finalizer.POD_FINALIZER_NAME)
		if err := r.Update(ctx, currentPod); err != nil {
			if apierrors.IsNotFound(err) {
				// Pod may already be gone - return and don't requeue the request
				return ctrl.Result{}, nil
			}

			// error updating the Pod - return and requeue
			return ctrl.Result{}, err
		}

		// write pod tracking info to all configured backends
		if errs := r.writePodInfo(ctx, &tracking.PodInfoConfig{
			Pod:   currentPod,
			Node:  currentNode,
			Event: tracking.PodDeleteEvent,
		}); len(errs) > 0 {
			// writing to one or more backends failed - return and requeue with error
			return ctrl.Result{}, errors.Join(errs...)
		}

		// pod deletion successfully recorded - return and don't requeue
		return ctrl.Result{}, nil
	}

	// ensure that the pod is in a state where we can get the necessary information from it
	if currentPod.Status.Phase != corev1.PodRunning {
		rl.V(2).Info(
			"Pod is not scheduled to a node yet",
		)

		// return and requeue with a small delay to give some time for the pod to get into a running state
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// add a finalizer to the pod so that we can intercept pod deletions to log deletion timestamp (establishes lifetime of Pod IP allocation)
	if !controllerutil.ContainsFinalizer(currentPod, finalizer.POD_FINALIZER_NAME) {
		controllerutil.AddFinalizer(currentPod, finalizer.POD_FINALIZER_NAME)
		if err := r.Update(ctx, currentPod); err != nil {
			if apierrors.IsNotFound(err) {
				// Pod was likely deleted before reconciliation - return and don't requeue
				return ctrl.Result{}, nil
			}

			// error updating the Pod - return and requeue
			return ctrl.Result{}, err
		}
	}

	// write pod tracking info to all configured backends
	if errs := r.writePodInfo(ctx, &tracking.PodInfoConfig{
		Pod:   currentPod,
		Node:  currentNode,
		Event: tracking.PodCreateEvent,
	}); len(errs) > 0 {
		// writing to one or more backends failed - return and requeue with error
		return ctrl.Result{}, errors.Join(errs...)
	}

	// reconciliation was successful - return and don't requeue
	return ctrl.Result{}, nil
}

// writePodInfo adds some additional context (such as which PodTracker CR has been configured to track this pod) to the provided PodInfo object
// and writes the provided PodInfo to all the configured writers
func (r *PodReconciler) writePodInfo(ctx context.Context, cfg *tracking.PodInfoConfig) (errs []error) {
	info := tracking.New(cfg)

	// aquire the cached config
	r.PodTrackerConfig.Lock()
	defer r.PodTrackerConfig.Unlock()

	for _, pt := range r.PodTrackerConfig.Items {
		if pt.TracksPod(cfg.Pod) {
			info.TrackedBy = pt.GetName()
			errs = writer.WriteToAll(pt.GetWriters(), info)
		}
	}

	return
}

// enqueueTrackedPods lets us ensure that only Pods that are tracked by a PodTracker configuration are reconciled.
// This helps to avoid errors where
func (r *PodReconciler) enqueueTrackedPods(ctx context.Context, obj client.Object) []reconcile.Request {
	// aquire the cached config
	r.PodTrackerConfig.Lock()
	defer r.PodTrackerConfig.Unlock()

	for _, pt := range r.PodTrackerConfig.Items {
		if pt.TracksPod(obj) {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			}
		}
	}

	return []reconcile.Request{}
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("PodTracker-Pod").
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueTrackedPods),
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc: func(ce event.CreateEvent) bool { return true },
					UpdateFunc: func(ue event.UpdateEvent) bool {
						// only enqueue pod updates that have a podtracker finalizer and are being deleted
						return !ue.ObjectNew.GetDeletionTimestamp().IsZero() && controllerutil.ContainsFinalizer(ue.ObjectNew, finalizer.POD_FINALIZER_NAME)
					},
					DeleteFunc:  func(de event.DeleteEvent) bool { return false },
					GenericFunc: func(ge event.GenericEvent) bool { return false },
				},
			),
		).
		Complete(r)
}
