package main

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	optInLabel    = "demo.io/welcome"
	configMapName = "welcome"
)

// NamespaceReconciler makes sure every Namespace labeled demo.io/welcome=true
// contains a "welcome" ConfigMap, and recreates it if someone deletes or edits it.
type NamespaceReconciler struct {
	client.Client
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ns corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ns.Labels[optInLabel] != "true" || !ns.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: ns.Name},
	}

	// CreateOrUpdate fetches the object, runs the mutate func, then creates or
	// updates only if something actually changed.
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = map[string]string{
			"message":   "Hello from the namespace controller!",
			"namespace": ns.Name,
		}
		// Owner reference: the ConfigMap is garbage-collected with the Namespace,
		// and changes to it trigger a reconcile of the owning Namespace (via Owns below).
		return controllerutil.SetControllerReference(&ns, cm, r.Scheme())
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		logger.Info("configmap reconciled", "operation", op)
	}
	return ctrl.Result{}, nil
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Owns(&corev1.ConfigMap{}). // re-reconcile if our ConfigMap is edited/deleted
		Complete(&NamespaceReconciler{Client: mgr.GetClient()}); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
