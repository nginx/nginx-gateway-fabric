output "region" {
  value       = data.google_client_config.current.region
  description = "GCloud Region"
}

output "k8s_cluster_zone" {
  value       = google_container_cluster.primary.location
  description = "GKE Cluster Zone"
}

output "project_id" {
  value       = data.google_client_config.current.project
  description = "GCloud Project ID"
  sensitive   = true
}

output "k8s_cluster_name" {
  value       = google_container_cluster.primary.name
  description = "GKE Cluster Name"
}

output "k8s_cluster_version" {
  value       = google_container_cluster.primary.master_version
  description = "GKE Cluster Version"
}

output "vm_name" {
  value       = google_compute_instance.vm.name
  description = "VM Name"
}
