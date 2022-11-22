# Google Cloud Storage Cache

GCS cache is an abstraction on top of Google Cloud Storage (GCS) which implements the tegola cache interface. To use it, you need to configure cache as the example below:

```toml
[cache]
# required
type="gcs"
bucket="your_bucket_name"   # Bucket is the name of the GCS bucket to operate on

# optional
basepath="tegola"           # Basepath is a path prefix added to all cache operations inside of the GCS bucket
max_zoom=8                  # MaxZoom determines the max zoom the cache to persist.
```

The credentials (service account and project_id) are handled by the `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
