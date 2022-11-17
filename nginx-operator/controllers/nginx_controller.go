/*
Copyright 2022.

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cachev1alpha1 "github.com/example/nginx-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// NginxReconciler reconciles a Nginx object
type NginxReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.example.com,resources=nginxes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.example.com,resources=nginxes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.example.com,resources=nginxes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Nginx object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *NginxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nginx", req.NamespacedName)

	// TODO(user): your logic here

	// Fetch the nginx instance
	nginx := &cachev1alpha1.Nginx{}
	err := r.Get(ctx, req.NamespacedName, nginx)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Nginx resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Nginx")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: nginx.Name, Namespace: nginx.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForNginx(nginx)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)

		if err != nil {
			log.Error(err, "Failed to create new deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	// Ensure the deployment replicaCount is the same as the spec
	replicaCount := nginx.Spec.ReplicaCount
	if *found.Spec.Replicas != replicaCount {
		found.Spec.Replicas = &replicaCount
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// check if the service already exists, if not: create a new one
	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: nginx.Name, Namespace: nginx.Namespace}, foundService)
	if err != nil && errors.IsNotFound(err) {
		// Create the Service
		newService := r.serviceForNginx(nginx)
		log.Info("Creating a new service", "Service.Namespace", newService.Namespace, "Service.Name", newService.Name)
		err = r.Create(ctx, newService)
		if err != nil {
			log.Error(err, "Failed to create new service", "Service.Namespace", newService.Namespace, "Service.Name", newService.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed tp get Service")
		return ctrl.Result{}, err
	}
	// Ensure the Service port is the same as the spec
	port := nginx.Spec.Port
	if foundService.Spec.Ports[0].Port != port {
		foundService.Spec.Ports[0].Port = port
		err = r.Update(ctx, foundService)
		if err != nil {
			log.Error(err, "Failed to update service", "Service.Namespace", foundService.Namespace, "Service.Name", foundService.Name)
			return ctrl.Result{}, err
		}
	}

	// Update the Nginx status with the pod names
	// List the pods for this nginx's deployment
	// podList := &corev1.PodList{}
	// listOpts := []client.ListOption{
	// 	client.InNamespace(nginx.Namespace),
	// 	client.MatchingLabels(labelsForNginx(nginx.Name)),
	// }
	// if err = r.List(ctx, podList, listOpts...); err != nil {
	// 	log.Error(err, "Failed to list pods", "Nginx.Namespace", nginx.Namespace, "Nginx.Name", nginx.Name)
	// 	return ctrl.Result{}, err
	// }
	// podName := getPodName(podList.Items)

	// // Update status.Nodes if needed
	// if !reflect.DeepEqual(podName, nginx.Status.Nodes) {
	// 	nginx.Status.Nodes = podName
	// 	err := r.Status().Update(ctx, nginx)
	// 	if err != nil {
	// 		log.Error(err, "Failed to update Nginx status")
	// 		return ctrl.Result{}, err
	// 	}
	// }
	return ctrl.Result{}, nil
}

// Create a Service for the Nginx sever.
func (r *NginxReconciler) serviceForNginx(nginxCR *cachev1alpha1.Nginx) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginxCR.Name,
			Namespace: nginxCR.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "ovh-nginx-server",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       nginxCR.Spec.Port,
					TargetPort: intstr.FromInt(80),
				},
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
	return service
}

// deploymentForNginx returns a nginx Deployment object
func (r *NginxReconciler) deploymentForNginx(n *cachev1alpha1.Nginx) *appsv1.Deployment {

	replicas := n.Spec.ReplicaCount

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "ovh-nginx-server"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "ovh-nginx-server"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "ovhplatform/hello:1.0",
						Name:  "ovh-nginx",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
							Protocol:      "TCP",
						}},
					}},
				},
			},
		},
	}
	// Set Nginx instance as the owner and controller
	// ctrl.SetControllerReference(n, dep, r.Scheme)
	return dep
}

// labelsForNginx returns the labels for selecting the resources
// belonging to the given nginx CR name.
// func labelsForNginx(name string) map[string]string {
// 	return map[string]string{"app": "nginx", "nginx_cr": name}
// }

// getPodNames retirns the pod names of the array of pods passed in
// func getPodName(pods []corev1.Pod) []string {
// 	var podName []string
// 	for _, pod := range pods {
// 		podName = append(podName, pod.Name)
// 	}
// 	return podName
// }

// SetupWithManager sets up the controller with the Manager.
func (r *NginxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Nginx{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(predicate.Funcs{
			// Check only delete events for a service
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		})).
		Complete(r)
}
