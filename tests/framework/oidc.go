package framework

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// formActionRegex matches the action attribute of an HTML form element.
var formActionRegex = regexp.MustCompile(`<form[^>]*\saction="([^"]*)"`)

// hiddenInputRegex matches hidden input fields and captures their name and value attributes.
var hiddenInputRegex = regexp.MustCompile(
	`<input\s+type="hidden"\s+name="([^"]*)"\s+value="([^"]*)"`,
)

// parseFormAction extracts the action URL from the first HTML form in the body.
func parseFormAction(body string) (string, error) {
	matches := formActionRegex.FindStringSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("no form action found in HTML body")
	}
	// Unescape HTML entities in the URL (Keycloak uses &amp; in form actions).
	action := strings.ReplaceAll(matches[1], "&amp;", "&")
	return action, nil
}

// parseHiddenInputs extracts all hidden input name=value pairs from the HTML body.
func parseHiddenInputs(body string) map[string]string {
	inputs := make(map[string]string)
	for _, match := range hiddenInputRegex.FindAllStringSubmatch(body, -1) {
		inputs[match[1]] = match[2]
	}
	return inputs
}

// oidcCurlOpts holds common parameters for in-cluster curl calls against an OIDC-protected NGINX service.
type oidcCurlOpts struct {
	k8sClient     kubernetes.Interface
	k8sConfig     *rest.Config
	namespace     string
	clientPodName string
	resolveArg    string // --resolve value, e.g. "cafe.example.com:443:10.96.0.5"
	cookieFile    string // path inside the pod for cookie persistence
}

// curlResult holds the result of a curl call.
type curlResult struct {
	Body       string
	FinalURL   string
	StatusCode int
}

// curlFollowRedirects runs a curl command in the client pod that follows redirects, writes the response
// body to outputFile, and returns the HTTP status code and response body.
// extraArgs are inserted before the URL (e.g., "-d", "username=...&password=...").
func curlFollowRedirects(
	ctx context.Context,
	opts oidcCurlOpts,
	targetURL,
	outputFile string,
	extraArgs ...string,
) (int, string, error) {
	res, err := curlFollowRedirectsFull(ctx, opts, targetURL, outputFile, extraArgs...)
	if err != nil {
		return 0, "", err
	}
	return res.StatusCode, res.Body, nil
}

// curlFollowRedirectsFull is like curlFollowRedirects but also returns the final URL after redirects.
func curlFollowRedirectsFull(
	ctx context.Context,
	opts oidcCurlOpts,
	targetURL,
	outputFile string,
	extraArgs ...string,
) (curlResult, error) {
	cmd := []string{
		"curl", "-sSk",
		"--resolve", opts.resolveArg,
		"-b", opts.cookieFile,
		"-c", opts.cookieFile,
		"-L",
	}
	cmd = append(cmd, extraArgs...)
	// Use write-out to get both http_code and url_effective, separated by a delimiter.
	cmd = append(cmd, "-o", outputFile, "-w", "%{http_code}|%{url_effective}", targetURL)

	result, err := execInPod(ctx, opts.k8sClient, opts.k8sConfig, opts.namespace, opts.clientPodName, "client", cmd)
	if err != nil {
		return curlResult{}, fmt.Errorf("curl %s failed: %w (stdout: %s)", targetURL, err, result.Stdout)
	}

	// Parse "http_code|url_effective" from stdout.
	parts := strings.SplitN(strings.TrimSpace(result.Stdout), "|", 2)
	httpCode := parts[0]
	finalURL := ""
	if len(parts) > 1 {
		finalURL = parts[1]
	}

	GinkgoWriter.Printf("OIDC in-cluster: %s -> %s (final: %s)\n", targetURL, httpCode, finalURL)

	catResult, err := execInPod(
		ctx, opts.k8sClient, opts.k8sConfig, opts.namespace, opts.clientPodName, "client",
		[]string{"cat", outputFile},
	)
	if err != nil {
		return curlResult{}, fmt.Errorf("failed to read response from %s: %w", outputFile, err)
	}

	var statusCode int
	if _, err := fmt.Sscanf(httpCode, "%d", &statusCode); err != nil {
		return curlResult{}, fmt.Errorf("failed to parse status code %q: %w", httpCode, err)
	}

	return curlResult{
		StatusCode: statusCode,
		Body:       catResult.Stdout,
		FinalURL:   finalURL,
	}, nil
}

// newOIDCCurlOpts builds the shared curl options for an OIDC flow against the given app path.
func newOIDCCurlOpts(
	k8sClient kubernetes.Interface,
	k8sConfig *rest.Config,
	namespace,
	clientPodName,
	nginxService,
	appHost,
	appPath string,
) oidcCurlOpts {
	suffix := strings.TrimLeft(appPath, "/")
	return oidcCurlOpts{
		k8sClient:     k8sClient,
		k8sConfig:     k8sConfig,
		namespace:     namespace,
		clientPodName: clientPodName,
		resolveArg:    fmt.Sprintf("%s:443:%s", appHost, nginxService),
		cookieFile:    fmt.Sprintf("/tmp/cookies-%s.txt", suffix),
	}
}

// PerformOIDCLoginInCluster performs an OIDC Authorization Code flow from inside the Kubernetes cluster
// by executing curl commands in a client pod. This avoids port-forwarding issues where Keycloak redirects
// would use in-cluster addresses that aren't reachable from the local machine.
//
// nginxService is the in-cluster NGINX service DNS name (e.g., "auth-gateway-nginx.namespace.svc").
// appHost is the hostname used in the HTTPRoute (e.g., "cafe.example.com").
// appPath is the protected path (e.g., "/coffee").
func PerformOIDCLoginInCluster(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	k8sConfig *rest.Config,
	namespace,
	clientPodName,
	nginxService,
	appHost,
	appPath,
	username,
	password string,
) (int, string, error) {
	opts := newOIDCCurlOpts(k8sClient, k8sConfig, namespace, clientPodName, nginxService, appHost, appPath)
	suffix := strings.TrimLeft(appPath, "/")

	appURL := fmt.Sprintf("https://%s%s", appHost, appPath)
	loginPageFile := fmt.Sprintf("/tmp/login-page-%s.html", suffix)

	// Step 1: GET the protected resource. Follow redirects (-L) to reach the Keycloak login page.
	// The initial request has no cookies yet, so we only write (-c) without reading (-b).
	step1Cmd := []string{
		"curl", "-sSk",
		"--resolve", opts.resolveArg,
		"-c", opts.cookieFile,
		"-L",
		"-o", loginPageFile,
		"-w", "%{url_effective}",
		appURL,
	}

	GinkgoWriter.Printf("OIDC in-cluster: Step 1 - GET %s\n", appURL)

	result, err := execInPod(ctx, k8sClient, k8sConfig, namespace, clientPodName, "client", step1Cmd)
	if err != nil {
		return 0, "", fmt.Errorf("step 1 (GET protected resource) failed: %w (stdout: %s)", err, result.Stdout)
	}

	GinkgoWriter.Printf("OIDC in-cluster: Step 1 final URL: %s\n", result.Stdout)

	// Read the login page HTML to extract the form action.
	catResult, err := execInPod(ctx, k8sClient, k8sConfig, namespace, clientPodName, "client",
		[]string{"cat", loginPageFile},
	)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read login page: %w", err)
	}

	formAction, err := parseFormAction(catResult.Stdout)
	if err != nil {
		return 0, "", fmt.Errorf(
			"failed to parse form action from login page: %w\nPage content:\n%s", err, catResult.Stdout,
		)
	}

	GinkgoWriter.Printf("OIDC in-cluster: Step 2 - POST credentials to %s\n", formAction)

	// Step 2: POST the login credentials to the form action URL.
	// Follow redirects back through the OIDC callback on NGINX.
	return curlFollowRedirects(
		ctx,
		opts,
		formAction,
		fmt.Sprintf("/tmp/final-response-%s.html", suffix),
		"-d", fmt.Sprintf("username=%s&password=%s", username, password),
	)
}

// PerformOIDCLogoutInCluster performs an OIDC RP-Initiated Logout from inside the Kubernetes cluster.
// The flow is:
//  1. GET the logout URI on NGINX, which redirects to Keycloak's end_session_endpoint.
//  2. Keycloak returns a "Do you want to log out?" confirmation page with a form.
//  3. Parse the form action and hidden session_code, then POST to confirm logout.
//
// logoutPath is the path that triggers logout (e.g., "/logout").
// appPath is the original protected path used during login (used to derive cookie file names).
func PerformOIDCLogoutInCluster(
	ctx context.Context,
	k8sClient kubernetes.Interface,
	k8sConfig *rest.Config,
	namespace,
	clientPodName,
	nginxService,
	appHost,
	logoutPath,
	appPath string,
) (int, string, error) {
	opts := newOIDCCurlOpts(k8sClient, k8sConfig, namespace, clientPodName, nginxService, appHost, appPath)
	suffix := strings.TrimLeft(appPath, "/")
	logoutURL := fmt.Sprintf("https://%s%s", appHost, logoutPath)
	logoutPageFile := fmt.Sprintf("/tmp/logout-page-%s.html", suffix)

	// Step 1: GET the logout URI. NGINX redirects to Keycloak's end_session_endpoint,
	// which returns a logout confirmation page.
	GinkgoWriter.Printf("OIDC in-cluster: Logout Step 1 - GET %s\n", logoutURL)

	step1Result, err := curlFollowRedirectsFull(ctx, opts, logoutURL, logoutPageFile)
	if err != nil {
		return 0, "", fmt.Errorf("logout step 1 (GET logout URI) failed: %w", err)
	}
	if step1Result.StatusCode != http.StatusOK {
		return step1Result.StatusCode, step1Result.Body,
			fmt.Errorf("expected logout confirmation page (200), got %d", step1Result.StatusCode)
	}

	// Step 2: Parse the confirmation form and POST to it.
	formAction, err := parseFormAction(step1Result.Body)
	if err != nil {
		return 0, "", fmt.Errorf(
			"failed to parse logout confirmation form: %w\nPage content:\n%s", err, step1Result.Body,
		)
	}

	// The form action may be a relative path (e.g., "/realms/..."). Resolve it against the final URL
	// from step 1 (which is on Keycloak) to get the full URL.
	if !strings.HasPrefix(formAction, "http") {
		baseURL, parseErr := url.Parse(step1Result.FinalURL)
		if parseErr != nil {
			return 0, "", fmt.Errorf("failed to parse final URL %q: %w", step1Result.FinalURL, parseErr)
		}
		resolvedURL, resolveErr := baseURL.Parse(formAction)
		if resolveErr != nil {
			return 0, "", fmt.Errorf("failed to resolve form action %q: %w", formAction, resolveErr)
		}
		formAction = resolvedURL.String()
	}

	// Build POST data from hidden inputs (e.g., session_code).
	hiddenInputs := parseHiddenInputs(step1Result.Body)
	var postParts []string
	for k, v := range hiddenInputs {
		postParts = append(postParts, fmt.Sprintf("%s=%s", k, v))
	}
	postData := strings.Join(postParts, "&")

	GinkgoWriter.Printf("OIDC in-cluster: Logout Step 2 - POST confirmation to %s\n", formAction)

	return curlFollowRedirects(
		ctx, opts, formAction,
		fmt.Sprintf("/tmp/logout-response-%s.html", suffix),
		"-d", postData,
	)
}
