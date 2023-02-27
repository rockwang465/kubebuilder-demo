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

package controllers

import (
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"kubebuilder/controllers/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ingressv1beta1 "kubebuilder/api/v1beta1"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ingress.baiding.tech,resources=apps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ingress.baiding.tech,resources=apps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ingress.baiding.tech,resources=apps/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the App object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fmt.Println("join to Reconcile ===========>")
	// 1.2 实例化logger和app
	logger := log.FromContext(ctx)
	// TODO(user): your logic here
	app := &ingressv1beta1.App{}
	// 1.3 从缓存中获取app
	err := r.Get(ctx, req.NamespacedName, app)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// deployment/ingress/service资源的yaml文件基于go template模板生成
	// 之前我们创建资源对象时，都是通过构造golang的struct来构造，但是对于复杂的资源对象 这样做费时费力;
	// 所以，我们可以先将资源定义为go template，然后替换需要修改的值之后， 反序列化为golang的struct对象，
	// 然后再通过client-go帮助我们创建或更新指定的资源。
	// 我们的deployment、service、ingress都放在了controllers/template中，通过 utils来完成上述过程。

	// 根据app的配置进行处理
	// 1.4.1 Deployment的处理
	// 1.4.1.1 生成deployment yaml模板
	deployment := utils.NewDeployment(app)
	fmt.Printf("deployment: ==> %#v\n", deployment)

	// 1.4.1.2 设置为OwnerReference
	// SetControllerReference将owner设置为Controller OwnerReference。
	// 这用于受控对象的垃圾收集，以及调整所有者对象对受控对象的更改(使用Watch + EnqueueRequestForOwner)。
	// 由于只有一个OwnerReference可以是控制器，如果有另一个OwnerReference设置了controller标志，它将返回一个错误。
	err = controllerutil.SetControllerReference(app, deployment, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 1.4.1.3 查找同名的deployment，不存在则创建，存在则更新
	d := &v1.Deployment{}
	//err = r.Get(ctx, req.NamespacedName, d)
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, d)
	if err != nil {
		// 如果不存在则创建
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, deployment); err != nil {
				logger.Error(err, "create deployment failed")
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// 如果存在则更新

		//Bug: 这里会反复触发更新
		//原因：在148行SetupWithManager方法中，监听了Deployment，所以只要更新Deployment就会触发
		//     此处更新和controllerManager更新Deployment都会触发更新事件，导致循环触发
		//修复方法：
		//方式1. 注释掉在148行SetupWithManager方法中对Deployment，Ingress，Service等的监听，该处的处理只是为了
		//      手动删除Deployment等后能够自动重建，但正常不会出现这种情况，是否需要根据情况而定
		//方式2. 加上判断条件，仅在app.Spec.Replicas != deployment.Spec.Replicas &&
		//      app.Spec.Image != deployment.Spec.Template.Spec.Containers[0].Image时才更新deployment
		if err = r.Update(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1.4.2 Service的处理
	// 生成模板、设置OwnerReference
	service := utils.NewService(app)
	fmt.Printf("service: ==> %#v\n", service)
	err = controllerutil.SetControllerReference(app, service, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 查找同名的service，不存在则创建，存在则更新
	s := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, s)
	if err != nil {
		// 如果不存在则创建
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, service); err != nil {
				logger.Error(err, "create service failed")
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// 如果存在则更新
		if err = r.Update(ctx, service); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 1.4.3 Ingress的处理
	// TODO 使用admission校验该值,如果启用了ingress，那么service必须启用
	// TODO 使用admission设置默认值,默认为false
	// Fix: 这里会导致Ingress无法被删除
	// 生成模板、设置OwnerReference
	if !app.Spec.EnableService {
		return ctrl.Result{}, err
	}

	ingress := utils.NewIngress(app)
	fmt.Printf("ingress: ==> %#v\n", ingress)
	err = controllerutil.SetControllerReference(app, ingress, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 查找同名的ingress，不存在则创建，存在则更新
	i := &netv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, i)
	if err != nil {
		// 如果不存在则创建
		if errors.IsNotFound(err) && app.Spec.EnableIngress {
			if err = r.Create(ctx, ingress); err != nil {
				logger.Error(err, "create ingress failed")
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		if app.Spec.EnableIngress {
			// 如果存在则更新
			if err = r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			} else {
				logger.Error(err, "update ingress failed")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 1.5 资源watch
	// 需要监测哪个资源,就要用Owns()去watch哪个资源.
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressv1beta1.App{}).
		Owns(&v1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&netv1.Ingress{}).
		Complete(r)
}
