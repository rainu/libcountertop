package smburl

import "testing"

func TestReferencesStayWithinConfiguredShare(t *testing.T) {
	for _, raw := range []string{"smb://media/video.mp4", "smb://media/folder/a%20b.mp4", "smb://media/日本語.mp4"} {
		p, e := Parse(raw)
		if e != nil || !Path(p) {
			t.Errorf("rejected %q", raw)
		}
	}
	for _, raw := range []string{"smb://user:secret@media/a.mp4", "smb://other/a.mp4", "smb://media:445/a.mp4", "smb://media/../a.mp4", "smb://media/%2e%2e/a.mp4", "smb://media/a%5cb.mp4", "smb://media/a:b.mp4", "smb://media/a.mp4?password=x", "smb://media/a.mp4#x", "smb://media/", "smb://media//a.mp4", "smb://media/%252e%252e/a", "smb://media/a./b", "smb://media/a%00b", "smb://media/a?", "smb://media/a%2f..%2fb"} {
		if _, e := Parse(raw); e == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
