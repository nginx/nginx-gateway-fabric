package validation

import (
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
)

// HTTPValidator validates values that will propagate into the NGINX configuration http context.
// The validation rules are based on the nginx/config/http types and how they are used in the configuration templates
// of the nginx/config package. Changes to those might require changing the validation rules.
type HTTPValidator struct {
	HTTPNJSMatchValidator
	HTTPRedirectValidator
	HTTPURLRewriteValidator
	HTTPHeaderValidator
	HTTPPathValidator
}

func (HTTPValidator) SkipValidation() bool { return false }

var _ validation.HTTPFieldsValidator = HTTPValidator{}
