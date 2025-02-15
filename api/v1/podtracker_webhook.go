package v1

import (
	"github.com/gccloudone-aurora/podtracker/internal/writer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var podtrackerlog = logf.Log.WithName("podtracker-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *PodTracker) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-networking-aurora-gc-ca-v1-podtracker,mutating=true,failurePolicy=fail,sideEffects=None,groups=networking.aurora.gc.ca,resources=podtrackers,verbs=create;update,versions=v1,name=mpodtracker.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &PodTracker{}

// Default implements webhook.Defaulter to apply default values to a PodTracker CR via a Mutating webhook
func (r *PodTracker) Default() {
	podtrackerlog.Info("default", "name", r.Name)

	// set the default backend writer to 'stdout' and enable it
	r.Spec.BackendWriterConfig = writer.BackendWriterConfig{
		Stdout: &writer.StdoutConfig{
			Enabled: true,
		},
	}
}

//+kubebuilder:webhook:path=/validate-networking-aurora-gc-ca-v1-podtracker,mutating=false,failurePolicy=fail,sideEffects=None,groups=networking.aurora.gc.ca,resources=podtrackers,verbs=create;update,versions=v1,name=vpodtracker.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &PodTracker{}

// ValidateCreate implements webhook.Validator to validate PodTracker CR creation via a validating webhook
func (r *PodTracker) ValidateCreate() (admission.Warnings, error) {
	podtrackerlog.Info("validate create", "name", r.Name)
	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator to validate PodTracker CR creation via a validating webhook
func (r *PodTracker) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	podtrackerlog.Info("validate update", "name", r.Name)
	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type. We do not need to validate anything upon deletion for this type, so we do nothing here
func (r *PodTracker) ValidateDelete() (admission.Warnings, error) {
	podtrackerlog.Info("validate delete", "name", r.Name)
	return nil, nil
}

func (r PodTracker) validate() error {
	var errs field.ErrorList
	if err := r.validateSpec(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: "networking.aurora.gc.ca", Kind: "PodTracker"}, r.Name, errs)
}

func (r PodTracker) validateSpec() *field.Error {
	if len(r.Spec.NSToWatch) == 0 {
		return field.Invalid(field.NewPath("spec").Child("nsToWatch"), r.Spec.NSToWatch, "Must specify at least one namespace that this PodTracker applies to")
	}
	return nil
}
