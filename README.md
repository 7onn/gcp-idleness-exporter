# gcp-idle-resources-metrics
Identify unused resources at Google Cloud Platform through Prometheus' metrics

## Current supported services
- Google Compute Engine
  - Instances
  - Disks

## Usage

Set up a service account on the project you want to monitor. You must grant `roles/compute.viewer` to it.

You can authenticate by setting the [Application Default Credentials](https://developers.google.com/accounts/docs/application-default-credentials) (i.e: Placing the service account's JSON key and setting the environment variable `GOOGLE_APPLICATION_CREDENTIALS=path-to-credentials.json`) or letting the application automatically load the credentials from metadata ([Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) is recommended).

You must set, at least, the project ID and the regions you want to monitor. Either by: 
- Specifying through command args `--project_id  --regions us-east1,us-central1`  
- Specifying through environment variables `GCP_PROJECT_ID= GCP_REGIONS=us-east1,us-central1` (if authenticating through metadata, the project doesn't need to be specified)


### Docker
```bash
cp ~/.config/gcloud/application_default_credentials.json ./credentials.json

chmod 444 credentials.json

docker build -t gcp-idle-resources-metrics . 

docker run -it --rm --network=host \
  -v $(pwd)/credentials.json:/credentials.json \
  -e GOOGLE_APPLICATION_CREDENTIALS=/credentials.json \
  -e GCP_PROJECT_ID= \
  -e GCP_REGIONS=us-east1,us-central1,southamerica-east1 \
  gcp-idle-resources-metrics
```
Check the exported [metrics](http://localhost:5000/metrics).


### Kubernetes
Add the Chart repository
```bash
helm repo add 7onn https://www.7onn.dev/helm-charts
helm search repo 7onn
```

Export its default values
```bash
helm show values 7onn/gcp-idle-resources-metrics > values.yaml
```

Edit the values according to your needs then install the application
```bash
helm upgrade -i gcp-idle-resources-metrics --values values.yaml 7onn/gcp-idle-resources-metrics
```
