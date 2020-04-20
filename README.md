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


**Create bucket**
```bash
curl  -X POST 'localhost:8080/api/v1/objects?type=training-data&name=mybucket'  -H "Authorization: sage <sage_user_token>"
```

Example response:
```json5
{
  "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "name": "mybucket",
  "owner": "user-auth-disabled",
  "type": "training-data"
}
```

**Show bucket**

```bash
curl 'localhost:8080/api/v1/objects/{bucket_id}'  -H "Authorization: sage <sage_user_token>"
```

Example response:
```json5
{
  "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "name": "mybucket",
  "owner": "user-auth-disabled",
  "type": "training-data",
  "time_created": "2020-04-20T18:34:09Z",
  "time_last_updated": "2020-04-20T18:34:09Z"
}
```

**List bucket/folder content**

List of files and folders at a given path within the bucket:
```bash
curl 'localhost:8080/api/v1/objects/{bucket_id}/{path}/'  -H "Authorization: sage <sage_user_token>"
```

Example response:
```json5
[
  "20200122-1403_1579730602.jpg"
]
```

Note that to get a listing of the bucket/folder content a `/` is required at the end or the path. 


TODO: add quer `?recursive`


**List buckets**
```bash
curl 'localhost:8080/api/v1/objects'  -H "Authorization: sage <sage_user_token>"
```

Example response:
```json5
[
  {
    "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
    "name": "mybucket",
    "owner": "user-auth-disabled",
    "type": "training-data"
  },
  {
    "id": "5f77bb1e-242f-4222-8eba-6c2c20b71b5e",
    "name": "mybucket2",
    "owner": "user-auth-disabled",
    "type": "training-data"
  }
]
```
This list should include all buckets that are either public, your own, or have been shared with you.


**Bucket permissions**

```bash
curl 'localhost:8080/api/v1/objects/{bucket_id}/?permissions' -H "Authorization: sage <sage_user_token>"
```

```json5
[
  {
    "user": "user-auth-disabled",
    "permission": "FULL_CONTROL"
  }
]
```

TODO: add/remove permissions

**Upload file**
```bash
curl  -X PUT 'localhost:8080/api/v1/objects/{bucket_id}/{path}'  -H "Authorization: sage <sage_user_token>" -F 'file=@<filename>'
```
Example response:
```json5
{
  "bucket-id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "key": "/20200122-1403_1579730602.jpg"
}
```

Similar to S3 keys, the path is an identifer for the uploaded file. The path can contain `/`-characters, thus creating a filesystem-like tree structure within the SAGE bucket. If the path ends with a `/`, the path denotes a directory and the filename of the uploaded file is appended to the key. Otherwise the last part of the part specifies the new filename.



**Download file**
```bash
curl -O 'localhost:8080/api/v1/objects/{bucket_id}/{key}'  -H "Authorization: sage <sage_user_token>" 
```


