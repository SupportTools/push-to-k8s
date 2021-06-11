# push-to-k8s
Pushing common assets to all namespaces in kubernetes cluster 

## Install
- Create namespace
```
kubectl create namespace push-to-k8s
```
- Deploy script as configmap
```
kubectl -n push-to-k8s create configmap push-to-k8s --from-file=main.sh
```
- Deploy workload
```
kubectl -n push-to-k8s apply -f workload.yaml
```