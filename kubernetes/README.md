

```bash
kubectl create namespace sage

# permanently save the namespace for all subsequent kubectl commands in that context.
kubectl config set-context --current --namespace=sage

minikube addons enable ingress
```


# MinIO

```bash
helm install --namespace sage -f ./helm-minio.yaml sage-minio   stable/minio

kubectl apply -f sage-storage-minio.ingress

```

# SAGE storage API
```bash
kubectl create configmap sage-storage-db-initdb-config --from-file=../init.sql -n sage

kubectl kustomize . | kubectl apply -f -

```




