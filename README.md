# sage-restapi


The SAGE object store API is a frontend to a S3-style storage.

# Concepts:

## SAGE bucket

Each file (or group of files of same type) are stored in a SAGE bucket. Each upload of a new file (without specifying an existing bucket) creates a new bucket. Each SAGE bucket is created with an universally unique identifier (UUID).

Ownership and permissions are bucket specific. A large collection of files of the same type that belong together are intended to share one bucket. An example in context of SAGE would be a large training dataset of pictures. 

Note that SAGE buckets do not correspond S3 buckets in the backend. They are merely an abstraction layer to prevent conflicts in namespaces. (In the actual S3 backend all SAGE objects are spread randomly over 256 S3-buckets and every SAGE key is prefixed with the SAGE bucket uuid)

## Data types

Each SAGE bucket contains one or more files of the same data type. Currently `model` and `training-data` are supported. The data type concept is still evolving and thus more types, metadata schema and type vaildation may be introduced later.
Note that the query string `type=<type>` is required on creation of a bucket / upload of a file into a non-existing bucket.


## Authentication 

SAGE users authenticate via tokens they can get from mthe SAGE website.

curl example:
```bash
-H "Authorization: sage <sage_user_token>"
```




# Getting started

Pull or build image:
```bash
docker pull sagecontinuum/sage-restapi
docker build -t sagecontinuum/sage-restapi:latest .
```

```bash
docker-compose up
```

TODO: The docker-compose environment requires the sage-ui introspection api. Either add this or disable token vaildation for testing purposes.


# Usage

Upload file
```bash
curl  -X POST 'localhost:8080/api/v1/objects/?type=model'  -H "Authorization: sage <sage_user_token>" -F 'file=@<filename>'
```

Download file
```bash
curl -O 'localhost:8080/api/v1/objects/{bucket_id}/{key}'  -H "Authorization: sage <sage_user_token>" 
```

TODO: add examples

- create empty bucket
- upload files to existing bucket
- list public/private buckets 


# Temporary: direct bucket access

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
