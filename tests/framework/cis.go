package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
)

// CIS is F5's Container Ingress Services controller, also known as k8s-bigip-ctlr. It runs in the
// cluster and programs a BIG-IP from the IngressLink resource that NGF creates, using AS3. It is
// installed from its public Helm chart.
const (
	CISHelmRepoName = "f5-stable"
	CISHelmRepoURL  = "https://f5networks.github.io/charts/stable"
	CISChart        = "f5-stable/f5-bigip-ctlr"
	CISReleaseName  = "cis"
	CISNamespace    = "cis"
)

// CISConfig holds the values needed to install CIS against a specific BIG-IP.
type CISConfig struct {
	// BIGIPAddress is the BIG-IP management address that CIS connects to. CIS runs as a pod inside
	// the cluster, so it must use the BIG-IP internal subnet address. It cannot use the external NAT
	// address, which is only reachable from outside the GCP network.
	BIGIPAddress string
	// BIGIPMgmtPort is the iControl REST port that CIS connects to. On the single-NIC GCP VE this is
	// 8443. When it is empty the address is used without a port, which lets CIS fall back to its own
	// default of 443.
	BIGIPMgmtPort  string
	BIGIPPartition string
	BIGIPUsername  string
	BIGIPPassword  string
	// PoolMemberType is either nodeport or cluster. In nodeport mode the BIG-IP targets the node IP
	// and node port. In cluster mode it targets the pod IP directly.
	PoolMemberType string
	// EnableIPAM turns on CIS IPAM so an ExternalLoadBalancer using ipamLabel is allocated a VIP.
	EnableIPAM bool
}

// InstallCIS adds the F5 Helm repo and installs the f5-bigip-ctlr chart pointed at the given BIG-IP.
// The chart creates the BIG-IP login Secret from the username and password in cfg.
func InstallCIS(cfg CISConfig) ([]byte, error) {
	GinkgoWriter.Printf("Adding Helm repo %q (%s)\n", CISHelmRepoName, CISHelmRepoURL)
	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "add", CISHelmRepoName, CISHelmRepoURL, "--force-update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error adding CIS helm repo: %w", err)
	}

	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error updating helm repos: %w", err)
	}

	poolMemberType := cfg.PoolMemberType
	if poolMemberType == "" {
		poolMemberType = "nodeport"
	}

	bigipURL := cfg.BIGIPAddress
	if cfg.BIGIPMgmtPort != "" {
		bigipURL = net.JoinHostPort(cfg.BIGIPAddress, cfg.BIGIPMgmtPort)
	}

	args := []string{
		"install",
		CISReleaseName,
		CISChart,
		"--create-namespace",
		"--namespace", CISNamespace,
		"--set", "bigip_secret.create=true",
		"--set", "bigip_secret.username=" + cfg.BIGIPUsername,
		"--set", "bigip_secret.password=" + cfg.BIGIPPassword,
		"--set", "bigip_login_ip=" + bigipURL,
		"--set", "rbac.create=true",
		"--set", "serviceAccount.create=true",
		"--set", "args.bigip_url=" + bigipURL,
		"--set", "args.bigip_partition=" + cfg.BIGIPPartition,
		"--set", "args.pool_member_type=" + poolMemberType,
		"--set", "args.insecure=true",
		"--set", "args.log_level=DEBUG",
		"--set", "args.custom_resource_mode=true",
		"--wait",
	}

	// Turn on CIS IPAM so an ExternalLoadBalancer using ipamLabel is allocated a VIP.
	if cfg.EnableIPAM {
		args = append(args, "--set", "args.ipam=true")
	}

	GinkgoWriter.Printf(
		"Installing CIS (release=%s, namespace=%s, poolMemberType=%s, ipam=%t)\n",
		CISReleaseName, CISNamespace, poolMemberType, cfg.EnableIPAM,
	)

	return exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
}

// UninstallCIS uninstalls the CIS controller Helm release.
func UninstallCIS() ([]byte, error) {
	args := []string{"uninstall", CISReleaseName, "--namespace", CISNamespace}
	GinkgoWriter.Printf("Uninstalling CIS with command: helm %v\n", strings.Join(args, " "))

	return exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
}

// DeleteAS3Tenant removes the AS3 tenant CIS created on the BIG-IP, which deletes every virtual
// server and pool CIS programmed in that partition. CIS does not always remove these itself on
// teardown, especially for IPAM-allocated resources, so this keeps repeated runs against the same
// BIG-IP from accumulating stale objects. The tenant name is the BIG-IP partition.
func DeleteAS3Tenant(address, port, username, password, tenant string) error {
	mgmtPort := port
	if mgmtPort == "" {
		mgmtPort = "443"
	}
	url := fmt.Sprintf("https://%s/mgmt/shared/appsvcs/declare/%s", net.JoinHostPort(address, mgmtPort), tenant)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("building AS3 delete request: %w", err)
	}
	req.SetBasicAuth(username, password)

	// The test BIG-IP uses a self-signed certificate, so skip verification.
	tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // self-signed test cert
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return fmt.Errorf("deleting AS3 tenant %q: %w", tenant, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("deleting AS3 tenant %q: unexpected status %d", tenant, resp.StatusCode)
	}
	return nil
}
