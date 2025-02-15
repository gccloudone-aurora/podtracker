package v1

import (
	"github.com/gccloudone-aurora/podtracker/internal/writer"
	"github.com/gobwas/glob"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// namespaceIsWatched takes a namespace, a list of (globbable) namespaces (nsToWatch)
// and returns true if the provided namespace matches any of the namespaces in nsToWatch
func namespaceIsWatched(namespace string, nsToWatch []string) bool {
	for _, ns := range nsToWatch {
		g := glob.MustCompile(ns)
		if g.Match(namespace) {
			return true
		}
	}
	return false
}

// PodTrackerSpec defines configuration options for the PodTracker controller
type PodTrackerSpec struct {
	// NSToWatch is a list of namespaces where Pods should be watched and logged.
	// If empty, PodTracker will watch all namespaces
	//+optional
	NSToWatch []string `json:"nsToWatch,omitempty" patchStrategy:"merge"`

	// BackendWriterConfig configures one or many BackendWriter for PodTracker to use
	// A BackendWriter will take the structured PodInfo from pod create/delete events and transforms and writes them to some log/output backend
	//
	// Currently, the following backends are supported:
	//   - stdout: writes all data to stdout on the controller pod
	//
	//+optional
	BackendWriterConfig writer.BackendWriterConfig `json:"backendWriterConfig,omitempty"`
}

// PodTrackerStatus defines the observed state of PodTracker
type PodTrackerStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// PodTracker is the Schema for the podtrackers API
type PodTracker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodTrackerSpec   `json:"spec,omitempty"`
	Status PodTrackerStatus `json:"status,omitempty"`
}

// GetWriters returns a list of objects that implement the writer.BackendWriter interface
func (p PodTracker) GetWriters() []writer.BackendWriter {
	return p.Spec.BackendWriterConfig.GetWriters()
}

// TracksPod returns true if the provided Pod is tracked by the PodTracker.
// A Pod is considered as being tracked by the calling PodTracker if the namespace that contains the Pod is in the configured `spec.nsToWatch`
func (p PodTracker) TracksPod(obj client.Object) bool {
	return namespaceIsWatched(obj.GetNamespace(), p.Spec.NSToWatch)
}

//+kubebuilder:object:root=true

// PodTrackerList contains a list of PodTracker
type PodTrackerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodTracker `json:"items"`
}

// Contains is a method of PodTrackerList that returns true as well as an index for the contained podtracker
// if the provided podTracker resource is contained within the calling PodTrackerList
func (pl *PodTrackerList) Contains(podTracker *PodTracker) (bool, int) {
	for i, pt := range pl.Items {
		if pt.GetName() == podTracker.GetName() {
			return true, i
		}
	}

	return false, 0
}

func init() {
	SchemeBuilder.Register(&PodTracker{}, &PodTrackerList{})
}
