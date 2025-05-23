/*
Package validation includes validators to validate values that will propagate to the NGINX configuration.

The validation rules prevent two cases:
(1) Invalid values. Such values will cause NGINX to fail to reload the configuration.
(2) Malicious values. Such values will cause NGINX to succeed to reload, but will configure NGINX maliciously, outside
of the NGF capabilities. For example, configuring NGINX to serve the contents of the file system of its container.

The validation rules are based on the types in the parent config package and how they are used in the NGINX
configuration templates. Changes to those might require changing the validation rules.

The rules are much looser for NGINX than for the Gateway API. However, some valid Gateway API values are not valid for
NGINX.
*/
package validation
