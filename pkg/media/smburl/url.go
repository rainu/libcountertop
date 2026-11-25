// Package smburl defines credential-free references to the configured media share.
package smburl

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

const Host = "smb.media.invalid"

var ErrURL = errors.New("Use smb://media/folder/video.mp4 without credentials, query, or traversal")

func Parse(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 4096 || u.Scheme != "smb" || u.Host != "media" || u.User != nil ||
		u.RawQuery != "" ||
		u.ForceQuery ||
		u.Fragment != "" ||
		u.Opaque != "" ||
		!strings.HasPrefix(u.Path, "/") {
		return "", ErrURL
	}
	p := strings.TrimPrefix(u.Path, "/")
	if !Path(p) {
		return "", ErrURL
	}
	return p, nil
}
func Path(p string) bool {
	if p == "" || len(p) > 2048 || strings.ContainsAny(p, "\\:*?\"<>|%") {
		return false
	}
	for _, c := range p {
		if unicode.IsControl(c) {
			return false
		}
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || strings.HasSuffix(s, ".") ||
			strings.HasSuffix(s, " ") {
			return false
		}
	}
	return true
}

// Internal addresses are handled only by a dedicated in-process transport.
// They never bind a listener or contain server names or credentials.
func Internal(raw string) (string, error) {
	p, e := Parse(raw)
	if e != nil {
		return "", e
	}
	return (&url.URL{Scheme: "https", Host: Host, Path: "/" + p}).String(), nil
}
