## Access metrics endpoint thorough a POST request
```
curl -X POST "http://localhost:8080/api/v1/metrics"
```

## Metrics
- sage_bucket_creation_errors_total: Number of errors during the sage bucket creation
- bucket_creation_total: Number of sage bucket creations
- file_upload_total: Number of file uploads
- file_download_byte_size_total: the number of bytes downloaded
