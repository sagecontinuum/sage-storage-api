# sage-restapi

Docker container usage:

```bash
$ docker build -t sagerestapi .
```

```bash
$ docker run -p 8080:8080 sagerestapi ENDPOINT ACCESSKEY SECRETKEY DIR
```