package schema

type Frontend struct {
	WebDAV   bool   `json:"webdav"`
	Password string `json:"password"`
}
