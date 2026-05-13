package config

import (
	"fmt"
)

const SocketBasePath = "unix:/var/run/nginx/"

func getSocketNameTLS(port int32, hostname string) string {
	if hostname == "" {
		return fmt.Sprintf("%s%d.sock", SocketBasePath, port)
	}

	return fmt.Sprintf("%s%s-%d.sock", SocketBasePath, hostname, port)
}

func getSocketNameTLSTerminate(port int32, hostname string) string {
	if hostname == "" {
		return fmt.Sprintf("%s%d-terminate.sock", SocketBasePath, port)
	}

	return fmt.Sprintf("%s%s-%d-terminate.sock", SocketBasePath, hostname, port)
}

func getSocketNameHTTPS(port int32) string {
	return fmt.Sprintf("%shttps%d.sock", SocketBasePath, port)
}

func getTLSPassthroughVarName(port int32) string {
	return fmt.Sprintf("$dest%d", port)
}
