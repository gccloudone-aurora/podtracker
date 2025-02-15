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

package cleaner

import (
	"context"
	"errors"
	"time"

	"github.com/gccloudone-aurora/podtracker/internal/config"
	"github.com/gccloudone-aurora/podtracker/internal/finalizer"
	"github.com/gccloudone-aurora/podtracker/internal/tracking"
	"github.com/gccloudone-aurora/podtracker/internal/writer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// PodCleaner is a runnable component which periodically (period defined by 'CleanInterval') looks for
// Pods with stuck finalizers and removes the PodTracker finalizer if the Pod is marked for deletion
type PodCleaner struct {
	client.Client
	CleanInterval time.Duration

	PodTrackerConfig *config.CachedPodTrackerConfig
}

// a blank assignment of PodCleaner as a manager.Runnable to ensure that the interface is implemented
var _ manager.Runnable = &PodCleaner{}

// Start spawns a goroutine which periodically lists pods with the PodTracker finalizer and removes them
// if they are marked for deletion.
//
// This goroutine is canceled when the provided parent context is cancelled
func (c *PodCleaner) Start(parentContext context.Context) error {
	cl := log.FromContext(parentContext)
	ticker := time.NewTicker(c.CleanInterval)

	cl.Info("starting pod cleaner", "period", c.CleanInterval)
	go func(ctx context.Context) {
		for {
			<-ticker.C
			c.PodTrackerConfig.Lock()

			pods := &corev1.PodList{}
			if err := c.Client.List(ctx, pods, client.MatchingFields{
				"spec.finalizers": finalizer.POD_FINALIZER_NAME,
			}); err != nil {
				cl.Error(err, "unable to list pods with finalizer fieldSelector")
			}

			for _, pod := range pods.Items {
				if !pod.GetDeletionTimestamp().IsZero() {
					node := corev1.Node{}
					if err := c.Client.Get(ctx, types.NamespacedName{
						Name:      pod.Spec.NodeName,
						Namespace: corev1.NamespaceAll,
					}, &node); err != nil {
						if !apierrors.IsNotFound(err) {
							cl.Error(
								err,
								"unable to get Node association for Pod. will try again on the next cleanup if not reconciled in the meantime",
								"pod", pod.GetName(),
								"namespace", pod.GetNamespace(),
							)
						}
					}

					info := tracking.New(&tracking.PodInfoConfig{
						Pod:   &pod,
						Node:  &node,
						Event: tracking.PodDeleteEvent,
					})

					for _, pt := range c.PodTrackerConfig.Items {
						if pt.TracksPod(&pod) {
							info.TrackedBy = pt.GetName()
							errs := writer.WriteToAll(pt.GetWriters(), info)

							if len(errs) > 0 {
								cl.Error(
									errors.Join(errs...),
									"unable to write final pod delete. will try again later",
									"pod", pod.GetName(),
									"namespace", pod.GetNamespace(),
								)
								continue
							}
						}
					}

					controllerutil.RemoveFinalizer(&pod, finalizer.POD_FINALIZER_NAME)
					if err := c.Client.Update(ctx, &pod); err != nil {
						cl.Error(err, "unable to remove finalizer from pod", "name", pod.GetName(), "namespace", pod.GetNamespace())
						continue
					}
				}
			}
			c.PodTrackerConfig.Unlock()
		}
	}(parentContext)

	return nil
}
