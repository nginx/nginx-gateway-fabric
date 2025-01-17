resource "google_compute_network" "vpc" {
  name                    = "${var.gke_cluster_name}-vpc"
  auto_create_subnetworks = "false"
  project                 = data.google_client_config.current.project
  enable_ula_internal_ipv6 = true
}

resource "google_compute_subnetwork" "subnet" {
  name                     = "${var.gke_cluster_name}-subnet"
  network                  = google_compute_network.vpc.self_link
  ip_cidr_range            = "10.113.0.0/20"
  private_ip_google_access = true
  stack_type = "IPV4_IPV6"
  ipv6_access_type = "INTERNAL"

  secondary_ip_range {
    ip_cidr_range = "10.201.0.0/16"
    range_name    = "subnet-services"
  }

  secondary_ip_range {
    ip_cidr_range = "10.202.0.0/18"
    range_name    = "subnet-pods"
  }

  log_config {
    metadata = "INCLUDE_ALL_METADATA"
  }
}

resource "google_compute_router" "router" {
  name    = "${var.gke_cluster_name}-router"
  network = google_compute_network.vpc.self_link
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.gke_cluster_name}-nat"
  router                             = google_compute_router.router.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    = google_compute_subnetwork.subnet.self_link
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }
}

resource "google_compute_firewall" "ssh" {
  name    = "${var.gke_cluster_name}-ssh"
  network = google_compute_network.vpc.self_link
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["${chomp(data.http.myip.response_body)}/32"]
}

resource "google_compute_address" "vpc-ip" {
  name         = "${var.gke_cluster_name}-vpc-ip"
  address_type = "EXTERNAL"
  network_tier = "PREMIUM"
}
