locals {
  kubeconfig_data = {
    apiVersion = "v1"
    kind       = "Config"
    preferences = {
      colors = true
    }
    current-context = google_container_cluster.primary.name
    contexts = [
      {
        name = google_container_cluster.primary.name
        context = {
          cluster = google_container_cluster.primary.name
          user    = google_container_cluster.primary.name
        }
      }
    ]
    clusters = [
      {
        name = google_container_cluster.primary.name
        cluster = {
          server                     = "https://${google_container_cluster.primary.private_cluster_config.0.private_endpoint}"
          certificate-authority-data = google_container_cluster.primary.master_auth[0].cluster_ca_certificate
        }
      }
    ]
    users = [
      {
        name = google_container_cluster.primary.name
        user = {
          exec = {
            apiVersion         = "client.authentication.k8s.io/v1beta1"
            command            = "gke-gcloud-auth-plugin"
            interactiveMode    = "Never"
            provideClusterInfo = true
          }
        }
      }
    ]
  }
  kubeconfig = yamlencode(local.kubeconfig_data)
}

data "cloudinit_config" "kubeconfig_setup" {
  gzip          = false
  base64_encode = false

  part {
    content_type = "text/cloud-config"
    content      = <<-EOF
    #cloud-config
    write_files:
      - path: /home/username/.kube/config
        content: ${indent(10, yamlencode(local.kubeconfig))}
        permissions: '0600'
        owner: username:username

    runcmd:
      - echo "Kubeconfig has been written to /home/username/.kube/config"
      - |
        sudo -i -u username bash -c "cd /home/username/nginx-gateway-fabric/tests && git fetch -pP --all && git checkout ${var.ngf_branch} && git pull"
        echo "Branch ${var.ngf_branch} has been checked out."
    EOF
  }
}
