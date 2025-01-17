variable "gke_project" {
  description = "The project ID where the GKE cluster will be created."
  type        = string
}

variable "gke_cluster_name" {
  description = "The name of the GKE cluster."
  type        = string
}

variable "gke_cluster_region" {
  description = "The zone where the GKE cluster will be created."
  type        = string
  default     = "us-west1"
}

variable "gke_machine_type" {
  description = "The type of machine to use for the nodes."
  type        = string
  default     = "e2-medium"
}

variable "gke_num_nodes" {
  description = "The number of nodes to create in the cluster."
  type        = number
  default     = 3
}

variable "gke_nodes_service_account" {
  description = "The service account to use for the nodes."
  type        = string
}

variable "vm_service_account" {
  description = "The service account to use for the VM."
  type        = string

}

variable "ngf_branch" {
  description = "The branch of the NGF repository to use."
  type        = string
  default     = "main"
}
