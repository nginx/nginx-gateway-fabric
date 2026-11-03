#!/bin/bash

set -e

source ~/vars.env

sudo apt-get -y update
sudo apt-get -y install ca-certificates curl gnupg gcc make git golang-go wrk gnuplot

# Add Google Cloud's apt repo for the gcloud CLI, the GKE auth plugin, and kubectl
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee -a /etc/apt/sources.list.d/google-cloud-sdk.list

sudo apt-get -y update
sudo apt-get -y install google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin kubectl

# Install Helm via its official install script (no official Debian/Ubuntu apt repo is maintained
# by the Helm project itself).
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | bash
