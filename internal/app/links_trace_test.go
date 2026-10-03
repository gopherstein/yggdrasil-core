package app

import "testing"

// Links no source gave are named in the answer's notice, since they may
// not open.
func TestTraceRecordsLinkChecks(t *testing.T) {
	tr := &turnTrace{}
	tr.linksChecked(2, 1, []string{"https://go.dev/blog/made-up"})
	meta := tr.meta()
	if meta.Steps[0].Text != "Checked the links; some didn't come from a source" ||
		meta.Notice != "These links didn't come from a source and may not work: https://go.dev/blog/made-up. Search the site instead if one doesn't open." {
		t.Fatalf("meta = %+v", meta)
	}
	tr = &turnTrace{}
	tr.linksChecked(2, 2, nil)
	if meta := tr.meta(); meta.Steps[0].Text != "Checked the links and replaced 2 guessed links" || meta.Notice != "" {
		t.Fatalf("meta = %+v", meta)
	}
	tr = &turnTrace{}
	tr.linksChecked(0, 0, nil)
	if meta := tr.meta(); meta.Steps[0].Text != "Checked that the links come from the sources" || meta.Notice != "" {
		t.Fatalf("meta = %+v", meta)
	}
}
