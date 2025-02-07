provider "google" {
  project = var.gke_project
  region  = var.gke_cluster_region
}

data "http" "myip" {
  url = "https://ipv4.icanhazip.com"
}

data "google_client_config" "current" {}

data "google_compute_zones" "available" {}

locals {
  google_zone = data.google_compute_zones.available.names[1]
}

resource "google_container_cluster" "primary" {
  name    = var.gke_cluster_name
  project = data.google_client_config.current.project

  location                 = local.google_zone
  initial_node_count       = 1
  remove_default_node_pool = true

  network    = google_compute_network.vpc.self_link
  subnetwork = google_compute_subnetwork.subnet.self_link
  node_config {
    service_account = var.gke_nodes_service_account
    kubelet_config {
      cpu_manager_policy                     = ""
      insecure_kubelet_readonly_port_enabled = "FALSE"
    }
  }

  logging_config {
    enable_components = ["SYSTEM_COMPONENTS", "WORKLOADS"]
  }

  deletion_protection = false
  resource_labels = {
    env = "ngf-tests"
  }

  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = "${chomp(data.http.myip.response_body)}/32"
      display_name = "local-ip"
    }
    cidr_blocks {
      cidr_block   = google_compute_subnetwork.subnet.ip_cidr_range
      display_name = "vpc"
    }
  }

  private_cluster_config {
    enable_private_nodes        = true
    enable_private_endpoint = false
    # private_endpoint_subnetwork = google_compute_subnetwork.subnet.self_link
    master_ipv4_cidr_block = "172.16.0.0/28"
  }
  ip_allocation_policy {
    stack_type = "IPV4_IPV6"
    cluster_secondary_range_name = google_compute_subnetwork.subnet.secondary_ip_range.1.range_name
    services_secondary_range_name = google_compute_subnetwork.subnet.secondary_ip_range.0.range_name
  }
  datapath_provider = "ADVANCED_DATAPATH"
}

resource "google_container_node_pool" "primary_nodes" {
  name       = "${var.gke_cluster_name}-nodes"
  cluster    = google_container_cluster.primary.id
  node_count = var.gke_num_nodes

  node_config {
    machine_type    = var.gke_machine_type
    service_account = var.gke_nodes_service_account
    metadata = {
      block-project-ssh-keys   = "TRUE"
      disable-legacy-endpoints = "true"
    }
    tags = ["ngf-tests-${var.gke_cluster_name}-nodes"]
    shielded_instance_config {
      enable_secure_boot = true
    }
    kubelet_config {
      cpu_manager_policy                     = ""
      insecure_kubelet_readonly_port_enabled = "FALSE"
    }
  }

  lifecycle {
    ignore_changes = [
      initial_node_count
    ]
  }

}

resource "google_compute_instance" "vm" {
  name                      = "${var.gke_cluster_name}-vm"
  machine_type              = "n2-standard-2"
  zone                      = local.google_zone
  allow_stopping_for_update = true
  tags                      = ["ngf-tests-${var.gke_cluster_name}-vm"]

  boot_disk {
    initialize_params {
      image = "ngf-debian"
    }
  }
  shielded_instance_config {
    enable_secure_boot = true
  }
  network_interface {
    network    = google_compute_network.vpc.self_link
    subnetwork = google_compute_subnetwork.subnet.self_link

    access_config {
      nat_ip = google_compute_address.vpc-ip.address
    }
  }

  service_account {
    email  = var.vm_service_account
    scopes = ["cloud-platform"]
  }
  metadata = {
    user-data              = data.cloudinit_config.kubeconfig_setup.rendered
    block-project-ssh-keys = "TRUE"
  }
}
