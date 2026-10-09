package runtime

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// FormatServerURL converts a listen address (e.g. ":8080", "127.0.0.1:8080", "0.0.0.0:8080")
// into a browser-accessible HTTP URL.
func FormatServerURL(listenAddr string) string {
	listenAddr = strings.TrimSpace(listenAddr)
	if strings.HasPrefix(listenAddr, "http://") || strings.HasPrefix(listenAddr, "https://") {
		return listenAddr
	}

	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		if strings.HasPrefix(listenAddr, ":") {
			return "http://localhost" + listenAddr
		}
		return "http://localhost:8080"
	}

	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}

	return fmt.Sprintf("http://%s:%s", host, port)
}

// OpenBrowser opens the specified URL in the default web browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
