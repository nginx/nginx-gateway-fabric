package validation

import (
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// HTTPValidator validates values that will propagate into the NGINX configuration http context.
// The validation rules are based on the nginx/config/http types and how they are used in the configuration templates
// of the nginx/config package. Changes to those might require changing the validation rules
type HTTPValidator struct {
	HTTPNJSMatchValidator
	HTTPRedirectValidator
	HTTPURLRewriteValidator
	HTTPRequestHeaderValidator
}

var _ validation.HTTPFieldsValidator = HTTPValidator{}
