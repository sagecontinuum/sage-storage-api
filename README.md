# SAGE storage api


The SAGE object store API is a frontend to a S3-style storage backend.

![Github Actions](https://github.com/sagecontinuum/sage-storage-api/workflows/docker-compose%20test/badge.svg?branch=master)


Related resources:

[Python client library for SAGE object store](https://github.com/sagecontinuum/sage-storage-py)

[SAGE CLI](https://github.com/sagecontinuum/sage-cli)


# Concepts:

## SAGE bucket

Each file (or group of files of same type) are stored in a SAGE bucket. Each upload of a new file (without specifying an existing bucket) creates a new bucket. Each SAGE bucket is created with an universally unique identifier (UUID).

Ownership and permissions are bucket specific. A large collection of files of the same type that belong together are intended to share one bucket. An example in context of SAGE would be a large training dataset of pictures. 

Note that SAGE buckets do not correspond S3 buckets in the backend. They are merely an abstraction layer to prevent conflicts in namespaces. (In the actual S3 backend all SAGE objects are spread randomly over 256 S3-buckets and every SAGE key is prefixed with the SAGE bucket uuid)

## Data types

Each SAGE bucket contains one or more files of the same data type. Currently `model` and `training-data` are supported. The data type concept is still evolving and thus more types, metadata schema and type validation may be introduced later.
Note that the query string `type=<type>` is required on creation of a bucket.


## Authentication 

SAGE users authenticate via tokens they can get from the SAGE website.

example:
```bash
-H "Authorization: sage <sage_user_token>"
```


In the docker-compose test environment the SAGE token verification is disabled. The `Authorization` header is still required, but the token field specifies a user name: `sage user:<username>`

example:
```bash
-H "Authorization: sage user:test"
```

To activate token verification in the test environment you can delete the file `.env` or define the environment variable `export TESTING_NOAUTH=0` before running docker-compose. You may have to update the `tokenInfo` variables in the `docker-compose.yaml` file.


# Getting started

```bash
docker-compose up
```

This starts a test environment without token verification.


# Usage

```
export SAGE_USER_TOKEN=<your_token>
or
export SAGE_USER_TOKEN=user:testuser
```

**Create bucket**
```bash
curl  -X POST 'localhost:8080/api/v1/objects?type=training-data&name=mybucket'  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Example response:
```json5
{
  "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "name": "mybucket",
  "owner": "testuser",
  "type": "training-data"
}
```

optional query fields:
```text
public=true
name=<human readable bucket name>
type=training-data|profile|model
```

Store the returned bucket id in an enviornment variable to simply copy-paste most of the following API examples:
```bash
export BUCKET_ID=<id>
```


**Show bucket properties**

```bash
curl "localhost:8080/api/v1/objects/${BUCKET_ID}"  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Example response:
```json5
{
  "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "name": "mybucket",
  "owner": "testuser",
  "type": "training-data",
  "time_created": "2020-04-20T18:34:09Z",
  "time_last_updated": "2020-04-20T18:34:09Z"
}
```

**List bucket/folder content**

List of files and folders at a given path within the bucket:
```bash
curl "localhost:8080/api/v1/objects/${BUCKET_ID}/"  -H "Authorization: sage ${SAGE_USER_TOKEN}"
curl "localhost:8080/api/v1/objects/${BUCKET_ID}/{path}/"  -H "Authorization: sage ${SAGE_USER_TOKEN}"

curl "localhost:8080/api/v1/objects/${BUCKET_ID}/?recursive"  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Example response:
```json5
[
  "20200122-1403_1579730602.jpg"
]
```

Note that to get a listing of the bucket/folder content a `/` is required at the end or the path. 

Optional query field:

```text
recursive=true   # if enabled, all files are listed 
```

**List buckets**
```bash
curl 'localhost:8080/api/v1/objects'  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Example response:
```json5
[
  {
    "id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
    "name": "mybucket",
    "owner": "testuser",
    "type": "training-data"
  },
  {
    "id": "5f77bb1e-242f-4222-8eba-6c2c20b71b5e",
    "name": "mybucket2",
    "owner": "testuser",
    "type": "training-data"
  }
]
```
This list should include all buckets that are either public, your own, or have been shared with you.


**Delete bucket**

```bash
curl -X DELETE "localhost:8080/api/v1/objects/${BUCKET_ID}"  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Example response:
```json5
{
  "deleted": [
    "5c9b9ff7-e3f3-4271-9649-70dddad02f28"
  ]
}
```

Note: This also deletes all files !

**Bucket permissions**

Get permissions:
```bash
curl "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions" -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

example result:
```json5
[
  {
    "granteeType": "USER",
    "grantee": "testuser",
    "permission": "FULL_CONTROL"
  }
]
```

Add permission: (Share private data with other user!)
```bash
curl -X PUT "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions" -d '{"granteeType": "USER", "grantee": "otheruser", "permission": "READ"}' -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

example result:
```json5
{
  "granteeType": "USER",
  "grantee": "otheruser",
  "permission": "READ"
}
```

Make bucket public:
```bash
curl -X PUT "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions" -d '{"granteeType": "GROUP", "grantee": "AllUsers", "permission": "READ"}' -H "Authorization: sage ${SAGE_USER_TOKEN}"
```
(To make bucket public the group `AllUsers` need `READ` permission. Other permissions are not allowed.)
 
example result:
```json5
{
  "granteeType": "GROUP",
  "grantee": "AllUsers",
  "permission": "READ"
}
```


Delete all permission of a grantee:
```bash
curl -X DELETE "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions&grantee=USER:otheruser" -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

Delete specific permission of a grantee:
```bash
curl -X DELETE "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions&grantee=USER:otheruser:READ" -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

example result:
```json5
{
  "deleted": [
    "USER:otheruser:READ"
  ]
}
```



**Update bucket properties**

```bash
curl -X PATCH "localhost:8080/api/v1/objects/${BUCKET_ID}" -d '{"name":"new-bucket-name"}'  -H "Authorization: sage ${SAGE_USER_TOKEN}"
```

```json5
{
  "id": "7cf0640d-7b58-4ffc-bb92-5063db62a91d",
  "name": "new-bucket-name",
  "owner": "testuser",
  "type": "training-data",
  "time_created": "2020-04-21T16:51:51Z",
  "time_last_updated": "2020-04-21T17:58:02Z"
}
```

Only fields `name` and `type` can be modified.

TODO: add user metadata (type-specific and free-form) for search functionality 


**Upload file**
```bash
curl  -X PUT "localhost:8080/api/v1/objects/${BUCKET_ID}/{path}"  -H "Authorization: sage ${SAGE_USER_TOKEN}" -F 'file=@<filename>'
```
Example response:
```json5
{
  "bucket-id": "5c9b9ff7-e3f3-4271-9649-70dddad02f28",
  "key": "/20200122-1403_1579730602.jpg"
}
```

Similar to S3 keys, the path is an identifer for the uploaded file. The path can contain `/`-characters, thus creating a filesystem-like tree structure within the SAGE bucket. If the path ends with a `/`, the path denotes a directory and the filename of the uploaded file is appended to the key. Otherwise the last part of the path specifies the new filename.



**Download file**

```bash
curl -O "localhost:8080/api/v1/objects/${BUCKET_ID}/{key}"  -H "Authorization: sage ${SAGE_USER_TOKEN}" 
```


# Testing


```bash
docker-compose build  &&  docker-compose run --rm --entrypoint=gotestsum sage-api --format testname
```

single test:
```bash
docker-compose build  &&  docker-compose run --rm --entrypoint=gotestsum sage-api --format testname -- -run TestDeleteFile
```