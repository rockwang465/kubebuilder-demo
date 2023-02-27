# 1 generate code
```
# kubebuilder init --domain baiding.tech
# kubebuilder create api --group ingress --version v1beta1 --kind App
```

# 2 start to use

## 2.1 configuration the AppSpec
```
# go mod tidy

# vim api/v1beta1/app_types.go
type AppSpec struct {
  // INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
// Important: Run "make" to regenerate code after modifying this file

  EnableIngress bool `json:"enable_ingress,omitempty"`
  EnableService bool `json:"enable_service"`
  Replicas    int32  `json:"replicas"`
  Image string    `json:"image"`
}
```

## 2.2 generate crd resources file
```
# go mod tidy
# make manifests
test -s /data/gopath/src/github.com/rockwang465.com/kubebuilder-demo/bin/controller-gen && /data/gopath/src/github.com/rockwang465.com/kubebuilder-demo/bin/controller-gen --version | grep -q v0.11.1 || \
GOBIN=/data/gopath/src/github.com/rockwang465.com/kubebuilder-demo/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1
/data/gopath/src/github.com/rockwang465.com/kubebuilder-demo/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

```

## 2.3 add template files
```
# mkdir controllers/template
# vim controllers/template/deployment.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.ObjectMeta.Name}}
  namespace: {{.ObjectMeta.Namespace}}
  labels:
    app: {{.ObjectMeta.Name}}
spec:
  replicas: {{.Spec.Replicas}}
  selector:
    matchLabels:
      app: {{.ObjectMeta.Name}}
  template:
    metadata:
      labels:
        app: {{.ObjectMeta.Name}}
    spec:
      containers:
        - name: {{.ObjectMeta.Name}}
          image: {{.Spec.Image}}
          ports:
            - containerPort: 8080

# vim controllers/template/service.yml
apiVersion: v1
kind: Service
metadata:
  name: {{.ObjectMeta.Name}}
  namespace: {{.ObjectMeta.Namespace}}
spec:
  selector:
    app: {{.ObjectMeta.Name}}
  ports:
    - name: http
      protocol: TCP
      port: 8080
      targetPort: 80

# vim controllers/template/ingress.yml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{.ObjectMeta.Name}}
  namespace: {{.ObjectMeta.Namespace}}
spec:
  rules:
    - host: {{.ObjectMeta.Name}}.baiding.tech
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{.ObjectMeta.Name}}
                port:
                  number: 8080
  ingressClassName: nginx
```

## 2.4 add logic in Reconcile
### 2.4.1 configuration business logic
```
# vim controller/app_controller.go
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    //1.App的处理
	logger := log.FromContext(ctx)
	app := &ingressv1beta1.App{}
	//从缓存中获取app
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	
	//2.Deployment的处理
    //之前我们创建资源对象时，都是通过构造golang的struct来构造，但是对于复杂的资源对象 这样做费时费力，所以，我们可以先将资源定义为go template，然后替换需要修改的值之后， 反序列号为golang的struct对象，然后再通过client-go帮助我们创建或更新指定的资源。
    //我们的deployment、service、ingress都放在了controllers/template中，通过 utils来完成上述过程。
	//根据app的配置进行处理
	deployment := utils.NewDeployment(app)
	if err := controllerutil.SetControllerReference(app, deployment, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	//查找同名deployment
	d := &v1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, d); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				logger.Error(err, "create deploy failed")
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.Update(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
	}
	
	//3.Service的处理
	service := utils.NewService(app)
	if err := controllerutil.SetControllerReference(app, service, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	//查找指定service
	s := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, s); err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableService {
			if err := r.Create(ctx, service); err != nil {
				logger.Error(err, "create service failed")
				return ctrl.Result{}, err
			}
		}
		//Fix: 这里还需要修复一下
	} else {
		if app.Spec.EnableService {
			//Fix: 当前情况下，不需要更新，结果始终都一样
			if err := r.Update(ctx, service); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(ctx, s); err != nil {
				return ctrl.Result{}, err
			}

		}
	}
	
	//4.Ingress的处理,ingress配置可能为空
	//TODO 使用admission校验该值,如果启用了ingress，那么service必须启用
	//TODO 使用admission设置默认值,默认为false
	//Fix: 这里会导致Ingress无法被删除
	if !app.Spec.EnableService {
		return ctrl.Result{}, nil
	}
	ingress := utils.NewIngress(app)
	if err := controllerutil.SetControllerReference(app, ingress, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	i := &netv1.Ingress{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, i); err != nil {
		if errors.IsNotFound(err) && app.Spec.EnableIngress {
			if err := r.Create(ctx, ingress); err != nil {
				logger.Error(err, "create service failed")
				return ctrl.Result{}, err
			}
		}
		//Fix: 这里还是需要重试一下
	} else {
		if app.Spec.EnableIngress {
            //Fix: 当前情况下，不需要更新，结果始终都一样
			if err := r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(ctx, i); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	... ...
```

### 2.4.2 watch resources
```
//删除service、ingress、deployment时，自动重建
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ingressv1beta1.App{}).
		Owns(&v1.Deployment{}).
		Owns(&netv1.Ingress{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
```

## 2.5 install traefik(未安装)
```
# helm repo add traefik https://helm.traefik.io/traefik
# cat <<EOF>> traefik_values.yaml
ingressClass:
  enabled: true
  isDefaultClass: true #指定为默认的ingress
EOF

# helm install traefik traefik/traefik -f traefik_values.yaml 
```

## 2.6 apply crd resource
```
# make install
/data/gopath/src/kubebuilder-demo-test/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
test -s /data/gopath/src/kubebuilder-demo-test/bin/kustomize || { curl -Ss "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash -s -- 3.8.7 /data/gopath/src/kubebuilder-demo-test/bin; }
{Version:kustomize/v3.8.7 GitCommit:ad092cc7a91c07fdf63a2e4b7f13fa588a39af4f BuildDate:2020-11-11T23:14:14Z GoOs:linux GoArch:amd64}
kustomize installed to /data/gopath/src/kubebuilder-demo-test/bin/kustomize
/data/gopath/src/kubebuilder-demo-test/bin/kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/apps.ingress.baiding.tech created

# kubectl get crd | grep baiding
apps.ingress.baiding.tech                                       2023-02-16T02:54:35Z
```

## 2.7 modify config
```
# vim config/samples/ingress_v1beta1_app.yaml
spec:
  # TODO(user): Add fields here
  image: nginx:latest
  replicas: 3
  enable_ingress: false # 会被修改为true
  enable_service: false # 成功
```

## 2.7 add rbac permission
```
# vim controllers/app_controller.go
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
```

## 2.8 apply app resource and run controller
```
# kubectl apply -f config/samples/ingress_v1beta1_app.yaml
# kubectl get apps.ingress.baiding.tech
NAME         AGE
app-sample   1m

# go run main.go
当前我卡在这里了，有报错

# kubectl get deploy
NAME         READY     UP-TO-DATE  AVAILABLE  AGE
app-sample   3/3       3           3          4m44s
```

## 2.9 test
```
# vim config/samples/ingress_v1beta1_app.yaml
spec:
  # TODO(user): Add fields here
  image: nginx:latest
  replicas: 2  # 将副本数改为2，来测试效果
  enable_ingress: true  # 开启ingress
  enable_service: true  # 开启service

# kubectl apply -f config/samples/ingress_v1beta1_app.yaml

# kubectl get deploy # 发现副本数会有更新
NAME         READY     UP-TO-DATE  AVAILABLE  AGE
app-sample   3/2       3           3          4m44s

# kubectl get svc  # service也马上生成了
NAME         TYPE       CLUSTER-IP     EXTERNAL-IP  PORT(S)   AGE
app-sample   ClusterIP  10.97.226.194  <none>       8080/TCP  6s

# kubectl get ingress  # ingress也马上生成了
NAME         TYPE       HOSTS                    ADDRESS     PORT(S)   AGE
app-sample   traefik    app-sample.baiding.tech              80        6s
```