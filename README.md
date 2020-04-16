# SAGE storage api


The SAGE object store API is a frontend to a S3-style storage backend.

# Concepts:

## SAGE bucket

Each file (or group of files of same type) are stored in a SAGE bucket. Each upload of a new file (without specifying an existing bucket) creates a new bucket. Each SAGE bucket is created with an universally unique identifier (UUID).

Ownership and permissions are bucket specific. A large collection of files of the same type that belong together are intended to share one bucket. An example in context of SAGE would be a large training dataset of pictures. 

Note that SAGE buckets do not correspond S3 buckets in the backend. They are merely an abstraction layer to prevent conflicts in namespaces. (In the actual S3 backend all SAGE objects are spread randomly over 256 S3-buckets and every SAGE key is prefixed with the SAGE bucket uuid)

## Data types

Each SAGE bucket contains one or more files of the same data type. Currently `model` and `training-data` are supported. The data type concept is still evolving and thus more types, metadata schema and type vaildation may be introduced later.
Note that the query string `type=<type>` is required on creation of a bucket / upload of a file into a non-existing bucket.


## Authentication 

SAGE users authenticate via tokens they can get from the SAGE website.

curl example:
```bash
-H "Authorization: sage <sage_user_token>"
```




# Getting started

```bash
docker-compose up
```

For testing purposes this docker-compose environment is configured without token verification. To activate token verification you can define the enviornment variable `export TESTING_NOAUTH=0` before running docker-compose. You may have to update the `tokenInfo` variables in the `docker-compose.yaml` file.

# Usage


Create bucket:
```bash
curl  -X POST 'localhost:8080/api/v1/objects/?type=training-data&name=mybucket'  -H "Authorization: sage <sage_user_token>"
```

response:
```json5
{
  "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "name": "mybucket",
  "owner": "user-auth-disabled",
  "type": "training-data"
}
```



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

