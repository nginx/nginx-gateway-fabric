---
docs: "DOCS-1439"
---

Pull the latest stable release of the NGINX Gateway Fabric chart:

   ```shell
   helm pull oci://ghcr.io/nginx/charts/nginx-gateway-fabric --untar
   cd nginx-gateway-fabric
   ```

   If you want the latest version from the **main** branch, add `--version 0.0.0-edge` to your pull command.
