package controller

import "fmt"

// CreateNginxResourceName creates the base resource name for all nginx resources
// created by the control plane.
func CreateNginxResourceName(gatewayName, gatewayClassName string) string {
	return fmt.Sprintf("%s-%s", gatewayName, gatewayClassName)
}
