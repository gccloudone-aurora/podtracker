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

package tracking

import (
	corev1 "k8s.io/api/core/v1"
)

// PodEvent describes what kind of change a Pod has undergone
type PodEvent string

const (
	PodCreateEvent PodEvent = "Create"
	PodDeleteEvent PodEvent = "Delete"

	timestampFormat string = "2006-01-02T15:04:05-0700"
)

type PodInfoConfig struct {
	Pod   *corev1.Pod
	Node  *corev1.Node
	Event PodEvent
}

// PodInfo describes a structured set of fields/data related to a pod that is compatible with PodTracker writers
type PodInfo struct {
	TrackedBy         string              `json:"trackedBy,omitempty"`
	ID                string              `json:"id"`
	Event             PodEvent            `json:"event"`
	Name              string              `json:"name"`
	Namespace         string              `json:"namespace"`
	Labels            map[string]string   `json:"labels"`
	Annotations       map[string]string   `json:"annotations"`
	CreationTimestamp string              `json:"creationTimestamp"`
	DeletionTimestamp string              `json:"deletionTimestamp"`
	PodIP             string              `json:"podIP"`
	Node              string              `json:"node"`
	NodeIPs           map[string][]string `json:"nodeIPs"`
}

// New creates a new PodInfo structure
func New(cfg *PodInfoConfig) *PodInfo {
	nodeIPs := make(map[string][]string, 0)
	for _, addr := range cfg.Node.Status.Addresses {
		nodeIPs[string(addr.Type)] = append(nodeIPs[string(addr.Type)], addr.Address)
	}

	podInfo := &PodInfo{
		ID:                string(cfg.Pod.GetUID()),
		Event:             cfg.Event,
		Name:              cfg.Pod.GetName(),
		Namespace:         cfg.Pod.GetNamespace(),
		Labels:            cfg.Pod.GetLabels(),
		Annotations:       cfg.Pod.GetAnnotations(),
		CreationTimestamp: cfg.Pod.GetCreationTimestamp().Format(timestampFormat),
		PodIP:             cfg.Pod.Status.PodIP,
		Node:              cfg.Pod.Spec.NodeName,
		NodeIPs:           nodeIPs,
	}

	// Only set the deletion timestamp field if the pod is being deleted
	if podInfo.Event == PodDeleteEvent {
		podInfo.DeletionTimestamp = cfg.Pod.GetDeletionTimestamp().Format(timestampFormat)
	}

	return podInfo
}
