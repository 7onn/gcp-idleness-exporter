# gcp-idle-resources-metrics
Identify unused resources at Google Cloud Platform through Prometheus' metrics

## Current supported services
- Google Compute Engine
  - Compute Instances
  - Compute Disks

## Usage

Set up a service account in the project you want to monitor. The account should be granted `roles/compute.viewer`.

You can authenticate by setting the [Application Default Credentials](https://developers.google.com/accounts/docs/application-default-credentials) (i.e: Placing the service account's JSON key and setting the environment variable `GOOGLE_APPLICATION_CREDENTIALS=path-to-credentials.json`) or letting the application automatically load the credentials from metadata ([Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) is recomended).

You must set, at least, the project ID and the regions you want to monitor. Either by: 
- Specifying through command args `--project_id xxx --regions us-east1,us-central1`  
- Specifying through environment variables `GCP_PROJECT_ID=xxx GCP_REGIONS=us-east1,us-central1` (if authenticating through metadata, the project don't need to specified)
