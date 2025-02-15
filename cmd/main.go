package main

import (
	"flag"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	networkingv1 "github.com/gccloudone-aurora/podtracker/api/v1"
	"github.com/gccloudone-aurora/podtracker/internal/cleaner"
	"github.com/gccloudone-aurora/podtracker/internal/config"
	"github.com/gccloudone-aurora/podtracker/internal/controller"
	//+kubebuilder:scaffold:imports
)

// lookupEnvOrDefault is a short helper function which provides a way to lookup environment variables and return a default if nothing is set
func lookupEnvOrDefault(key string, defaultValue string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return v
}

var (
	// leaderElectionID represents the lock/lease identity used in leader-election
	leaderElectionID string
	// metricsAddr represents the address that the metric endpoint binds to
	metricsAddr string
	// probeAddr Is the address that the health probe endpoint binds to
	probeAddr string
	// enableLeaderElection specifies whether or not leader election should be used for the controller manager
	enableLeaderElection bool
	// developmentLogging specifies whether to enable Development (Debug) logging for ZAP. Otherwise, Zap Production logging will be used
	developmentLogging bool
	// disableWebhooks specifies whether to disable the configuration and runtime elements default and validating webhooks
	disableWebhooks bool
	// disablePodCleaner specifies whether or not to run the pod cleaner (A GC runnable which periodically checks for stuck pods and unsticks them)
	disablePodCleaner bool
	// podCleanerIntervalSeconds specifies the period (in seconds) for which the pod cleaner will run
	podCleanerIntervalSeconds uint
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// Configure CLI arguments
	flag.StringVar(
		&metricsAddr,
		"metrics-bind-address",
		lookupEnvOrDefault("METRICS_BIND_ADDR", ":9003"),
		"The address the metric endpoint binds to.",
	)
	flag.StringVar(
		&probeAddr,
		"health-probe-bind-address",
		lookupEnvOrDefault("HEALTH_PROBE_BIND_ADDR", ":8081"),
		"The address the probe endpoint binds to.",
	)
	flag.StringVar(
		&leaderElectionID,
		"leader-election-id",
		lookupEnvOrDefault("LEADER_ELECTION_ID", "podtracker-leader.aurora.gc.ca"),
		"The identity to use for leader-election",
	)
	flag.BoolVar(&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)
	flag.BoolVar(
		&developmentLogging,
		"dev-logging",
		false,
		"Enable development logging",
	)
	flag.BoolVar(
		&disableWebhooks,
		"disable-webhooks",
		lookupEnvOrDefault("DISABLE_WEBHOOKS", "false") != "false",
		"Choose to disable default and validating webhook functionality of the operator",
	)
	flag.BoolVar(
		&disablePodCleaner,
		"disable-podcleaner",
		lookupEnvOrDefault("DISABLE_POD_CLEANER", "false") != "false",
		"Disables the Pod Cleaner (A GC runnable for cleaning up stuck pods)",
	)
	flag.UintVar(
		&podCleanerIntervalSeconds,
		"pod-cleaner-interval",
		600,
		"The period (in seconds) in which the Pod Cleaner should check for stuck pods",
	)

	opts := zap.Options{
		Development: developmentLogging,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Configure ZAP Logger
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: "/tmp/podtracker-webhook-server/serving-certs",
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// configure validating/defaulting webhooks
	if !disableWebhooks {
		if err = (&networkingv1.PodTracker{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "PodTracker")
			os.Exit(1)
		}
	}

	// a local in-memory cache for relevant PodTracker data
	var cachedPodTrackers config.CachedPodTrackerConfig

	if err = (&controller.PodTrackerReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		PodTrackerConfig: &cachedPodTrackers,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodTracker")
		os.Exit(1)
	}

	if err = (&controller.PodReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		PodTrackerConfig: &cachedPodTrackers,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Podtracker-Pod-Controller")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	managerContext := ctrl.SetupSignalHandler()

	if err := mgr.GetFieldIndexer().IndexField(managerContext, &corev1.Pod{}, "spec.finalizers", func(o client.Object) []string {
		pod := o.(*corev1.Pod)
		return pod.GetFinalizers()
	}); err != nil {
		setupLog.Error(err, "unable to setup field indexer for Pod finalizers")
	}

	if !disablePodCleaner {
		if err := mgr.Add(&cleaner.PodCleaner{
			Client:           mgr.GetClient(),
			CleanInterval:    time.Duration(podCleanerIntervalSeconds) * time.Second,
			PodTrackerConfig: &cachedPodTrackers,
		}); err != nil {
			setupLog.Error(err, "unable to add pod cleaner runnable to manager")
		}
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(managerContext); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
