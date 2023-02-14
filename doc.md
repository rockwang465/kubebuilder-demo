# 1 init command
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
```
