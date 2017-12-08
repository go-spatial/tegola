# S3Cache

s3cache is an abstration on top of Amazon Web Services (AWS) Simple Storage Service (S3) which implements the tegola cache interface. To use it, add the following minimum config to your tegola config file:

```toml
[cache]
type="s3"   
bucket="tegola-test-data"
```

## Properties
The s3cache config supports the following config properties:

- `bucket` (string): [Required] the name of the S3 bucket to use.
- `region` (string): [Optoinal] the region the bucket is in. Defaults to 'us-east-1'
- `aws_access_key_id` (string): [Optoinal] the AWS access key id to use.
- `aws_secret_access_key` (string): [Optoinal] the AWS secret access key to use.
- `max_zoom` (int): [Optional] the max zoom the cache should cache to. After this zoom, Set() calls will return before doing work.

## Credential chain
If the `aws_access_key_id` and `aws_secret_access_key` are not set, then the [credential provider chain](http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) will be used. The provider chain supports multiple methods for passing credentials, one of which is setting environment variables. For example:

```bash
$ export AWS_REGION=us-west-2
$ export AWS_ACCESS_KEY_ID=YOUR_AKID
$ export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_KEY
```
