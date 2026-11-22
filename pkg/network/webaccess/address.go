package webaccess

import (
	"net"
	"net/url"
	"strings"
)

const DAVPath = "/api/v1/webdav/"

func DAVURL(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if ip := net.ParseIP(address); ip != nil && strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	u, e := url.Parse("https://" + address)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return (&url.URL{Scheme: "https", Host: u.Host, Path: DAVPath}).String()
}
