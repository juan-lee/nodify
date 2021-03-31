package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kctlutil "k8s.io/kubectl/pkg/cmd/util"
	kctldrain "k8s.io/kubectl/pkg/drain"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeConditionHandlerReconciler reconciles a NodeConditionHandler object
type NodeConditionHandlerReconciler struct {
	client.Client
	Clientset *kubernetes.Clientset
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=nodes;pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes/status;pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=pods/eviction,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *NodeConditionHandlerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NodeConditionHandlerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nodes", req.NamespacedName)

	var node corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		log.Error(err, "unable to fetch Node")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeCondition, err := getMaintenanceCondition(&node)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch nodeCondition.Reason {
	case "None":
		log.Info("No maintenance required", "condition", nodeCondition)
		if node.Spec.Unschedulable {
			if err := r.uncordon(&node); err != nil {
				return ctrl.Result{}, err
			}
		}
	case "Freeze":
		log.Info("The Virtual Machine is scheduled to pause for a few seconds.", "condition", nodeCondition)
	case "Reboot", "Redeploy", "Prempt", "Terminate":
		log.Info("Maintenance required", "condition", nodeCondition)
		if err := r.cordonAndDrain(&node); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NodeConditionHandlerReconciler) cordonAndDrain(node *corev1.Node) error {
	log := r.Log.WithValues("node", node.Name)
	helper := newDrainHelper(r.Clientset, log)
	log.Info("Cordoning node")
	if err := kctldrain.RunCordonOrUncordon(helper, node, true); err != nil {
		return err
	}
	log.Info("Draining node")
	if err := kctldrain.RunNodeDrain(helper, node.Name); err != nil {
		log.Info("Errors draining node", "err", err)
	}
	return nil
}

func (r *NodeConditionHandlerReconciler) uncordon(node *corev1.Node) error {
	log := r.Log.WithValues("node", node.Name)
	helper := newDrainHelper(r.Clientset, r.Log)
	log.Info("Uncordoning node")
	if err := kctldrain.RunCordonOrUncordon(helper, node, false); err != nil {
		return err
	}
	return nil
}

func newDrainHelper(cs *kubernetes.Clientset, log logr.Logger) *kctldrain.Helper {
	return &kctldrain.Helper{
		Client:              cs,
		Force:               true,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1,
		Timeout:             60 * time.Second,
		OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
			verbStr := "Deleted"
			if usingEviction {
				verbStr = "Evicted"
			}
			log.Info(fmt.Sprintf("%s pod from Node", verbStr),
				"pod", fmt.Sprintf("%s/%s", pod.Name, pod.Namespace))
		},
		DryRunStrategy: kctlutil.DryRunNone,
		Out:            writer{log.Info},
		ErrOut:         writer{log.Info},
	}
}

func getMaintenanceCondition(node *corev1.Node) (*corev1.NodeCondition, error) {
	for n, condition := range node.Status.Conditions {
		if condition.Type == "MaintenanceScheduled" {
			return &node.Status.Conditions[n], nil
		}
	}
	return nil, errors.New("missing MaintenanceScheduled NodeCondition")
}

// writer implements io.Writer interface as a pass-through for logr.
type writer struct {
	logFunc func(msg string, args ...interface{})
}

// Write passes string(p) into writer's logFunc and always returns len(p)
func (w writer) Write(p []byte) (n int, err error) {
	w.logFunc("DrainHelper", "msg", string(p))
	return len(p), nil
}
