/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	viewv1 "github.com/bobuhiro11/markdown-view/api/v1"
	appsv1 "k8s.io/api/apps/v1"
)

// MarkdownViewReconciler reconciles a MarkdownView object
type MarkdownViewReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//! [rbac]
//+kubebuilder:rbac:groups=view.bobuhiro11.net,resources=markdownviews,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=view.bobuhiro11.net,resources=markdownviews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=view.bobuhiro11.net,resources=markdownviews/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;update;patch
//! [rbac]

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MarkdownView object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *MarkdownViewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var mdView viewv1.MarkdownView

	// If the CR is not found, there is nothing to do here.
	err := r.Get(ctx, req.NamespacedName, &mdView)
	if errors.IsNotFound(err) {
		r.removeMetrics(mdView)
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "unable to get MarkdownView", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// The CR is under deletion. So there is nothing to do here.
	if !mdView.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Create related resources.
	err = r.reconcileConfigMap(ctx, mdView)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.reconcileDeployment(ctx, mdView)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.reconcileService(ctx, mdView)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.updateStatus(ctx, mdView)
}

func (r *MarkdownViewReconciler) Reconcile_get(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var deployment appsv1.Deployment

	err := r.Get(
		ctx,
		client.ObjectKey{Namespace: "default", Name: "sample"},
		&deployment,
	)

	if err != nil {
		_ = fmt.Errorf("Failed to get deployment: #%v\n", err)
		return ctrl.Result{}, err
	}

	fmt.Printf("Got Deployment: %#v\n", deployment)
	return ctrl.Result{}, nil
}

func (r *MarkdownViewReconciler) Reconcile_list(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var services corev1.ServiceList
	err := r.List(ctx, &services, &client.ListOptions{
		Namespace:     "default",
		LabelSelector: labels.SelectorFromSet(map[string]string{"app": "sample"}),
	})

	if err != nil {
		_ = fmt.Errorf("Failed to list deployment: #%v\n", err)
		return ctrl.Result{}, err
	}

	fmt.Printf("List deployments:\n")
	for _, svc := range services.Items {
		fmt.Println(svc.Name)
	}

	return ctrl.Result{}, nil
}

func (r *MarkdownViewReconciler) Reconcile_pagination(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var services corev1.ServiceList
	token := ""

	for i := 0; ; i++ {
		err := r.List(ctx, &services, &client.ListOptions{
			Limit:    3,
			Continue: token,
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		fmt.Printf("Page %d:\n", i)
		for _, svc := range services.Items {
			fmt.Println(svc.Name)
		}
		fmt.Println()

		token = services.ListMeta.Continue
		if len(token) == 0 {
			return ctrl.Result{}, nil
		}
	}
}

func (r *MarkdownViewReconciler) Reconcile_create(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "nginx"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "nginx"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	err := r.Create(ctx, &dep)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MarkdownViewReconciler) Reconcile_createOrUpdate(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	svc := &corev1.Service{}
	svc.SetNamespace("default")
	svc.SetName("sample")

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		svc.Spec.Selector = map[string]string{"app": "nginx"}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(80),
			},
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		fmt.Printf("Executed CreateOrUpdate. Deployment: %s\n", op)
	}

	return ctrl.Result{}, nil
}

func (r *MarkdownViewReconciler) Reconcile_patchMerge(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Before
	var dep appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &dep)
	if err != nil {
		return ctrl.Result{}, err
	}

	patch := client.MergeFrom(&dep)

	// After
	newDep := dep.DeepCopy()
	newDep.Spec.Replicas = pointer.Int32(3)

	err = r.Patch(ctx, newDep, patch)

	return ctrl.Result{}, err
}

func (r *MarkdownViewReconciler) Reconcile_patchApply(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	patch.SetNamespace("default")
	patch.SetName("sample2")
	patch.UnstructuredContent()["spec"] = map[string]interface{}{
		"replicas": 2,
		"selector": map[string]interface{}{
			"matchLabels": map[string]string{
				"app": "nginx",
			},
		},
		"template": map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]string{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
				},
			},
		},
	}

	err := r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "client-sample",
		Force:        pointer.Bool(true),
	})

	return ctrl.Result{}, err
}

func (r *MarkdownViewReconciler) Reconcile_patchApplyConfig(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	dep := applyappsv1.Deployment("sample3", "default").
		WithSpec(applyappsv1.DeploymentSpec().
			WithReplicas(3).
			WithSelector(applymetav1.LabelSelector().WithMatchLabels(map[string]string{"app": "nginx"})).
			WithTemplate(applycorev1.PodTemplateSpec().
				WithLabels(map[string]string{"app": "nginx"}).
				WithSpec(applycorev1.PodSpec().
					WithContainers(applycorev1.Container().
						WithName("nginx").
						WithImage("nginx:latest"),
					),
				),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return ctrl.Result{}, err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current appsv1.Deployment
	err = r.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample3"}, &current)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	currApplyConfig, err := applyappsv1.ExtractDeployment(&current, "client-sample")
	if err != nil {
		return ctrl.Result{}, err
	}

	if equality.Semantic.DeepEqual(dep, currApplyConfig) {
		return ctrl.Result{}, nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "client-sample",
		Force:        pointer.Bool(true),
	})
	return ctrl.Result{}, err
}

func (r *MarkdownViewReconciler) Reconcile_deleteWithPreConditions(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var deploy appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: "default", Name: "sample"}, &deploy)
	if err != nil {
		return ctrl.Result{}, err
	}
	uid := deploy.GetUID()
	resourceVersion := deploy.GetResourceVersion()
	cond := metav1.Preconditions{
		UID:             &uid,
		ResourceVersion: &resourceVersion,
	}
	err = r.Delete(ctx, &deploy, &client.DeleteOptions{
		Preconditions: &cond,
	})
	return ctrl.Result{}, err
}

func (r *MarkdownViewReconciler) Reconcile_deleteAllOfDeployment(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	err := r.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace("default"))
	return ctrl.Result{}, err
}

// Store markdowns in ConfigMap (the name is markdowns-*) in same namespace.
func (r *MarkdownViewReconciler) reconcileConfigMap(ctx context.Context, mdView viewv1.MarkdownView) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	cm.SetNamespace(mdView.Namespace)
	cm.SetName("markdowns-" + mdView.Name)

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		for name, content := range mdView.Spec.Markdowns {
			cm.Data[name] = content
		}

		return ctrl.SetControllerReference(&mdView, cm, r.Scheme)
	})

	if err != nil {
		logger.Error(err, "unable to createOrUpdate ConfigMap")
		return err
	}

	if op != controllerutil.OperationResultNone {
		logger.Info("reconcile ConfigMap successfully", "op", op)
	}

	return nil
}

// Create deployment for viewer (the name is view-*) in same namespace.
func (r *MarkdownViewReconciler) reconcileDeployment(ctx context.Context, mdView viewv1.MarkdownView) error {
	logger := log.FromContext(ctx)

	// Get deployment name and container image name.
	depName := "viewer-" + mdView.Name
	viewerImage := "peaceiris/mdbook:latest"
	if len(mdView.Spec.ViewerImage) != 0 {
		viewerImage = mdView.Spec.ViewerImage
	}

	owner, err := controllerReference(mdView, r.Scheme)
	if err != nil {
		return err
	}

	dep := appsv1apply.Deployment(depName, mdView.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mdbook",
			"app.kubernetes.io/instance":   mdView.Name,
			"app.kubernetes.io/created-by": "markdown-view-controller",
		}).
		// Set 'mdView' as owner.
		WithOwnerReferences(owner).
		WithSpec(appsv1apply.DeploymentSpec().
			WithReplicas(mdView.Spec.Replicas).
			WithSelector(metav1apply.LabelSelector().WithMatchLabels(map[string]string{
				"app.kubernetes.io/name":       "mdbook",
				"app.kubernetes.io/instance":   mdView.Name,
				"app.kubernetes.io/created-by": "markdown-view-controller",
			})).
			WithTemplate(corev1apply.PodTemplateSpec().
				WithLabels(map[string]string{
					"app.kubernetes.io/name":       "mdbook",
					"app.kubernetes.io/instance":   mdView.Name,
					"app.kubernetes.io/created-by": "markdown-view-controller",
				}).
				WithSpec(corev1apply.PodSpec().
					WithContainers(corev1apply.Container().
						WithName("mdbook").
						WithImage(viewerImage).
						WithImagePullPolicy(corev1.PullIfNotPresent).
						WithCommand("mdbook").
						WithArgs("serve", "--hostname", "0.0.0.0").
						WithVolumeMounts(corev1apply.VolumeMount().
							WithName("markdowns").
							WithMountPath("/book/src"),
						).
						WithPorts(corev1apply.ContainerPort().
							WithName("http").
							WithProtocol(corev1.ProtocolTCP).
							WithContainerPort(3000),
						).
						WithLivenessProbe(corev1apply.Probe().
							WithHTTPGet(corev1apply.HTTPGetAction().
								WithPort(intstr.FromString("http")).
								WithPath("/").
								WithScheme(corev1.URISchemeHTTP),
							),
						).
						WithReadinessProbe(corev1apply.Probe().
							WithHTTPGet(corev1apply.HTTPGetAction().
								WithPort(intstr.FromString("http")).
								WithPath("/").
								WithScheme(corev1.URISchemeHTTP),
							),
						),
					).
					// Get markdowns in config map.
					WithVolumes(corev1apply.Volume().
						WithName("markdowns").
						WithConfigMap(corev1apply.ConfigMapVolumeSource().
							WithName("markdowns-" + mdView.Name),
						),
					),
				),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current appsv1.Deployment
	err = r.Get(ctx, client.ObjectKey{Namespace: mdView.Namespace, Name: depName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "markdown-view-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(dep, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "markdown-view-controller",
		Force:        pointer.Bool(true),
	})

	if err != nil {
		logger.Error(err, "unable to create or update Deployment")
		return err
	}
	logger.Info("reconcile Deployment successfully", "name", mdView.Name)
	return nil
}

// Create service for viewer.
func (r *MarkdownViewReconciler) reconcileService(ctx context.Context, mdView viewv1.MarkdownView) error {
	logger := log.FromContext(ctx)
	svcName := "viewer-" + mdView.Name

	owner, err := controllerReference(mdView, r.Scheme)
	if err != nil {
		return err
	}

	svc := corev1apply.Service(svcName, mdView.Namespace).
		WithLabels(map[string]string{
			"app.kubernetes.io/name":       "mdbook",
			"app.kubernetes.io/instance":   mdView.Name,
			"app.kubernetes.io/created-by": "markdown-view-controller",
		}).
		// Set 'mdView' as owner.
		WithOwnerReferences(owner).
		WithSpec(corev1apply.ServiceSpec().
			WithSelector(map[string]string{
				"app.kubernetes.io/name":       "mdbook",
				"app.kubernetes.io/instance":   mdView.Name,
				"app.kubernetes.io/created-by": "markdown-view-controller",
			}).
			WithType(corev1.ServiceTypeClusterIP).
			WithPorts(corev1apply.ServicePort().
				WithProtocol(corev1.ProtocolTCP).
				WithPort(80).
				WithTargetPort(intstr.FromInt(3000)),
			),
		)

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var current corev1.Service
	err = r.Get(ctx, client.ObjectKey{Namespace: mdView.Namespace, Name: svcName}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := corev1apply.ExtractService(&current, "markdown-view-controller")
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(svc, currApplyConfig) {
		return nil
	}

	err = r.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "markdown-view-controller",
		Force:        pointer.Bool(true),
	})
	if err != nil {
		logger.Error(err, "unable to create or update Service")
		return err
	}

	logger.Info("reconcile Service successfully", "name", mdView.Name)
	return nil
}
func (r *MarkdownViewReconciler) updateStatus(ctx context.Context, mdView viewv1.MarkdownView) (ctrl.Result, error) {
	var dep appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: mdView.Namespace, Name: "viewer-" + mdView.Name}, &dep)
	if err != nil {
		return ctrl.Result{}, err
	}

	var status viewv1.MarkdownViewStatus
	if dep.Status.AvailableReplicas == 0 {
		status = viewv1.MarkdownViewNotReady
	} else if dep.Status.AvailableReplicas == mdView.Spec.Replicas {
		status = viewv1.MarkdownViewHealthy
	} else {
		status = viewv1.MarkdownViewAvailable
	}

	if mdView.Status != status {
		mdView.Status = status
		r.setMetrics(mdView)
		err = r.Status().Update(ctx, &mdView)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if mdView.Status != viewv1.MarkdownViewHealthy {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MarkdownViewReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&viewv1.MarkdownView{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func controllerReference(mdView viewv1.MarkdownView, scheme *runtime.Scheme) (*metav1apply.OwnerReferenceApplyConfiguration, error) {
	gvk, err := apiutil.GVKForObject(&mdView, scheme)
	if err != nil {
		return nil, err
	}
	// For Server-Side Apply?
	ref := metav1apply.OwnerReference().
		WithAPIVersion(gvk.GroupVersion().String()).
		WithKind(gvk.Kind).
		WithName(mdView.Name).
		WithUID(mdView.GetUID()).
		WithBlockOwnerDeletion(true).
		WithController(true)
	return ref, nil
}

func (r *MarkdownViewReconciler) setMetrics(mdView viewv1.MarkdownView) {
	switch mdView.Status {
	case viewv1.MarkdownViewNotReady:
		NotReadyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(1)
		AvailableVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
		HealthyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
	case viewv1.MarkdownViewAvailable:
		NotReadyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
		AvailableVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(1)
		HealthyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
	case viewv1.MarkdownViewHealthy:
		NotReadyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
		AvailableVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(0)
		HealthyVec.WithLabelValues(mdView.Name, mdView.Namespace).Set(1)
	}
}

func (r *MarkdownViewReconciler) removeMetrics(mdView viewv1.MarkdownView) {
	NotReadyVec.DeleteLabelValues(mdView.Name, mdView.Namespace)
	AvailableVec.DeleteLabelValues(mdView.Name, mdView.Namespace)
	HealthyVec.DeleteLabelValues(mdView.Name, mdView.Namespace)
}
