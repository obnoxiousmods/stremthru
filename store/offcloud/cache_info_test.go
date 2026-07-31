package offcloud

import (
	"encoding/json"
	"testing"
)

// Offcloud documents size on every file returned by /api/cache/info. It was not
// parsed, so every file surfaced as size -1 and callers could not tell a feature
// from a sample inside a season pack - the one judgement a cache check exists to
// inform.
func TestCacheInfoKeepsTheFileSize(t *testing.T) {
	payload := `[{"cached":true,"files":[
		{"folder":[],"filename":"readme.txt","size":5000},
		{"folder":["Season 1"],"filename":"video.mkv","size":1024000000}
	]},{"cached":false,"files":[]}]`

	var data GetCacheInfoData
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("documented response must parse: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 results, got %d", len(data))
	}
	if !data[0].Cached || data[1].Cached {
		t.Fatal("cached flags mis-parsed")
	}
	if got := data[0].Files[1].Size; got != 1024000000 {
		t.Fatalf("expected the documented size, got %d", got)
	}
	if got := data[0].Files[1].Filename; got != "video.mkv" {
		t.Fatalf("filename mis-parsed: %q", got)
	}
	if got := data[0].Files[1].Folder; len(got) != 1 || got[0] != "Season 1" {
		t.Fatalf("folder path mis-parsed: %v", got)
	}
	if len(data[1].Files) != 0 {
		t.Fatal("an uncached entry must carry no files")
	}
}

// A response without sizes must still parse - the field is newer than the API.
func TestCacheInfoToleratesAMissingSize(t *testing.T) {
	var data GetCacheInfoData
	if err := json.Unmarshal([]byte(`[{"cached":true,"files":[{"folder":[],"filename":"a.mkv"}]}]`), &data); err != nil {
		t.Fatalf("older responses must still parse: %v", err)
	}
	if data[0].Files[0].Size != 0 {
		t.Fatalf("a missing size must read as zero, got %d", data[0].Files[0].Size)
	}
}
