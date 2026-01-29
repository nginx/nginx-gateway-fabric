# Results

## Test environment

NGINX Plus: false

NGINX Gateway Fabric:

- Commit: 4e96123a5dababa8a4d398ab997efb64ef8265a8
- Date: 2026-01-29T17:46:34Z
- Dirty: false

GKE Cluster:

- Node count: 3
- k8s version: v1.33.5-gke.2100000
- vCPUs per node: 16
- RAM per node: 65851520Ki
- Max pods per node: 110
- Zone: us-west1-b
- Instance Type: n2d-standard-16

## Traffic

HTTP:

```text
unable to connect to cafe.example.com:http Connection refused
```

HTTPS:

```text
unable to connect to cafe.example.com:https Connection refused
```


## Error Logs

