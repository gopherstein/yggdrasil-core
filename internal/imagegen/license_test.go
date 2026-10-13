package imagegen

import (
	"strings"
	"testing"
	"time"
)

// A model behind a license is set up only with the person's own token and
// their confirmation, which is kept; its gated file downloads with the
// token, and a refused download says to accept the license (#551).
func TestModelBehindALicense(t *testing.T) {
	k, ok := Lookup("flux2-klein-9b")
	if !ok || !k.NeedsLicense || k.Untested || k.LicenseURL == "" || k.Tier != "" || !k.Files[0].Gated {
		t.Fatalf("flux2-klein-9b = %+v", k)
	}
	for _, mem := range []int64{16 << 30, 64 << 30, 256 << 30} {
		if Recommend(mem).NeedsLicense {
			t.Fatalf("%d GB recommends a licensed model", mem>>30)
		}
	}

	f := newFixture(t)
	m := f.model(t)
	m.ID, m.Name, m.NeedsLicense, m.LicenseURL, m.LicenseAllows = "licensed", "Licensed", true, "https://example.test/license", "Non-commercial use."
	m.Files = append([]File(nil), m.Files...)
	m.Files[0].Gated = true
	f.gated = map[string]string{m.Files[0].Path: "hf_secret_token_value_123"}
	f.setup.Catalog = func() []Model { return []Model{m} }
	token := ""
	f.setup.Token = func() string { return token }

	if err := f.setup.Start("licensed"); err == nil || !strings.Contains(err.Error(), "Hugging Face token") {
		t.Fatalf("without a token: %v", err)
	}
	token = "hf_secret_token_value_123"
	if err := f.setup.Start("licensed"); err == nil || !strings.Contains(err.Error(), "confirm that your license") {
		t.Fatalf("without confirming: %v", err)
	}
	if err := f.setup.AcceptLicense("licensed", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	st := f.setup.Status()
	if !st.TokenSet || !st.Models[0].LicenseAccepted {
		t.Fatalf("status %+v", st)
	}
	if err := f.setup.Start("licensed"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, f.setup); j.Error != "" {
		t.Fatalf("setup failed: %s", j.Error)
	}
	if !f.setup.installed("licensed") {
		t.Fatal("not installed")
	}

	// A token whose account hasn't accepted the license is refused, and
	// the person is told what to do.
	f2 := newFixture(t)
	m2 := f2.model(t)
	m2.ID, m2.Name, m2.NeedsLicense, m2.LicenseURL, m2.LicenseAllows = "licensed", "Licensed", true, "https://example.test/license", "Non-commercial use."
	m2.Files = append([]File(nil), m2.Files...)
	m2.Files[0].Gated = true
	f2.gated = map[string]string{m2.Files[0].Path: "hf_the_right_token_000"}
	f2.setup.Catalog = func() []Model { return []Model{m2} }
	f2.setup.Token = func() string { return "hf_another_account_token" }
	_ = f2.setup.AcceptLicense("licensed", time.Now())
	if err := f2.setup.Start("licensed"); err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, f2.setup); !strings.Contains(j.Error, "accept its license at https://example.test/license") {
		t.Fatalf("refused download said %q", j.Error)
	}
}
