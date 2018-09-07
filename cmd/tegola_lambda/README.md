# tegola_lambda
Run tegola on AWS lambda. This implementation uses the native Go AWS Lambda rutime. There are a couple limitations to using lambda to run tegola:

- No connection pooling: The database connection will be rebuilt on every lambda request.
- No built in viewer: Lambda + API Gateway have limitations which restrict the configuration from being easily setup to support the built in viewer and return vector tiles.

The following steps assume you have a working tegola configuration. 

## Creating a new lambda function

### From the AWS console:
- Navigate to lambda and click "Create function". We're going to "Author from scratch".
  - Name: name your function whatever you like.
  - Runtime: Go 1.x.
  - Role: Depends on your environment. For the sake of this walk through we will "Create a new role from template"
  - Role Name: Up to you. `tegola` is a good role name. 
  - Policy Templates: Select "Basic Edge Lambda Permissions".
- Click "Create function".

## Preparing the function for upload
Lambda expects an archive with the function to be uploaded. In tegola's case we will be creating an archive of `tegola_lambda` and a `config.toml` file:

```bash
zip deployment.zip tegola_lambda config.toml
```

*Note: tegola will check for a config file named `config.toml` by default. This can be changed by setting the environment variable `TEGOLA_CONFIG`*

Back in the AWS console for the function that was created earlier, locate the section "Function Code" and click "Upload". Upload the `deployment.zip` archive you just created.

## Configuring the Lambda function
- Under "Function Code" change `Handler` to `tegola_lambda` or, if you compiled from source use the name of the binary if you named it differently.
- Under "Basic Settings" 
  - Memory: Adjust to your preference. Depending on the config.toml file you may need more or less memory for the function. 
  - Timeout: Set to max time (5 minutes).

## Setting up API Gateway
In order to access the Lambda function publicly and API gateway will need to be setup. 
- Under the "Designer" section of the lambda console click "API Gateway". A new section will appear in the Lambda function tree called "API Gateway". Click on the new section.
- Under the "Configure triggers" section click the "API" drop down and select "Create a new API".
  - API Name: something obvious (i.e. tegola)
  - Deployment Stage: this is how you manage different environments (i.e. dev / stage / prod or v1, v2, etc.). Enter `dev`.
  - Security: Choose "Open" or whatever matches your security preferences.
  - Save the configuration by clicking "Save" in the upper right corner of the screen.
  - Under the "API Gateway" section click the title of the gateway that was just created. This will launch the API gateway console the API that was just created so we can configure it accordingly.

### Configuring API Gateway
- Under the Resources section, click on the root resource, "/".
- Click on "Actions" and select "Create Resource". A panel for "New Child Resource" will show up on the right.
- Check "Configure as Proxy Resource".
- Click "Create Resource". The resource will be created and a new panel will show up for additional configuration for the "/{proxy+} - ANY - Setup".
  - Integration type: Lambda Function Proxy
  - Lambda Region: Select the region the Lambda function lives.
  - Lambda Function: The name of the function that was created earlier.
  - Use Default Timeout: Un-check and set to 29 seconds. This is currently the max that API gateway allows.
  - Click "Save". A dialog box with pop up asking to allow the API Gateway permission to the selected Lambda function. Click "OK".
- On the left, under the APIs section, locate "Settings" under the configured API.
  - Under "Binary Media Types" click "Add Binary Media Type".
  - Input `*/*` as the value. This is necessary as tegola returns protocol buffers, which are a binary format. Without this configuration API gateway will return the vector tile payloads as base64 encoded strings.
  - Click "Save Changes".
- Click on "Resources" under the APIs section for the configured API. 
  - Click "Actions" under the resources section. Under the "API Actions" section, click "Deploy API".
  - Deployment Stage: Select the previous configured stage (i.e. `dev`).
  - Click "Deploy"
- After the API is deployed a new panel will show on the right and across the top will be an "Invoke URL". Copy this URL into a browser and append `/capabilities` to the route. A JSON response with details about the tegola configuration should be returned. 

Tegola is now running on lambda!

## Building from source
Building `tegola_lambda` works the same way as building normal tegola with the exception that it must be built for linux. Navigate to the repository root then `cmd/tegola_lambda` and execute the following command:

```
GOOS=linux go build
```

The `tegola_lambda` binary will be created.
