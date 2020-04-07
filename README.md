# sage-restapi

Docker container usage
-------------
The docker image is hosted on [DockerHub](https://hub.docker.com/repository/docker/sagecontinuum/sage-restapi)

To build image:
```bash
docker build -t sagecontinuum/sage-restapi:latest .
```

To run container:
```bash
docker run -p 8080:8080 sagecontinuum/sage-restapi:latest ENDPOINT ACCESSKEY SECRETKEY APINAME APIPASSW
```

To push a new tag to this repository:
```bash
docker push sagecontinuum/sage-restapi:latest
```

Kubernetes Setup
-------------
The command deploys the REST API on the Nautilus cluster where `minio_accesskey`, `minio_secretkey`, `api_username` and `api_password` are provided by the user (in Secrets).

```bash
$ kubectl create -f sage-restapi.yaml
```

User side
-------------
For testing purposes, download and upload parts are being done on Nautilus. `TOKEN` is generated from https://sage.nautilus.optiputer.net/ by the user after being authenticated.

Get the existing Minio buckets:
```
curl --location --request GET 'sage-restapi.nautilus.optiputer.net/api/v1/buckets' \
 --header 'Content-Type: multipart/form-data' --form 'token={TOKEN}'
```

Download `object` from `bucket`:
```
curl --location --request GET 'sage-restapi.nautilus.optiputer.net/api/v1/buckets/{bucket}/{object}' \
--form 'token={TOKEN}'
```

Upload `uploadObject` to `targetBucket`:
```
curl --location --request POST 'sage-restapi.nautilus.optiputer.net/api/v1/bucket/' \
--header 'Content-Type: multipart/form-data' \
--form 'token={TOKEN}' \
--form 'bucket={targetBucket}' \
--form 'file=@{uploadObject}'
```