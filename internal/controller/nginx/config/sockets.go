package config

import (
	"fmt"
)

func getSocketNameTLS(port int32, hostname string) string {
	return fmt.Sprintf("unix:/var/run/nginx/%s-%d.sock", hostname, port)
}

func getSocketNameHTTPS(port int32) string {
	return fmt.Sprintf("unix:/var/run/nginx/https%d.sock", port)
}

func getTLSPassthroughVarName(port int32) string {
	return fmt.Sprintf("$dest%d", port)
}
