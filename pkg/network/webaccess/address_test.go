package webaccess

import "testing"

func TestDAVAddress(t *testing.T) {
	for _, v := range []struct{ host, want string }{{"192.0.2.40", "https://192.0.2.40/api/v1/webdav/"}, {"device.example", "https://device.example/api/v1/webdav/"}, {"::1", "https://[::1]/api/v1/webdav/"}, {"", ""}, {"user:password@device.example", ""}} {
		if got := DAVURL(v.host); got != v.want {
			t.Fatal(v.host, got)
		}
	}
}
