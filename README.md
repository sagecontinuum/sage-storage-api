# sage-restapi

Docker container usage
-------------
The docker image is hosted on

```bash
$ docker build -t sagecontinuum/sage-restapi:latest .
```

```bash
$ docker run -p 8080:8080 sagecontinuum/sage-restapi:latest ENDPOINT ACCESSKEY SECRETKEY
```

Kubernetes Setup
-------------
The command deploys the REST API on the Nautilus cluster where `minio_accesskey` and `minio_secretkey` are provided by the user (in Secrets).

```bash
$ kubectl create -f sage-restapi.yaml
```

User side
-------------
For testing purposes, download and upload parts are being done on Nautilus' persistence storage that was setup in **Kubernetes Setup**. For the upload part, the REST-API is looking in the persistance storage for the `uploadObject` so if it does not find the file, the API will return null.

Get the existing Minio buckets:
```
sage-restapi.nautilus.optiputer.net/api/v1/buckets
```
Download `object` from `bucket`:
```
sage-restapi.nautilus.optiputer.net/api/v1/buckets/{bucket}/{object}
```

Upload `uploadObject` to `targetBucket`:
```
sage-restapi.nautilus.optiputer.net/api/v1/bucket?bucket={targetBucket}&object={uploadObject}
```