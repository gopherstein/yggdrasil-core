package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/diagnostics"
	"github.com/yeixio/toskar-core/internal/logs"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A report has the kind of computer for anyone, and Core's log, scrubbed,
// only for an Admin (#588).
func TestReport(t *testing.T) {
	srv := NewServer(Dependencies{
		Hardware: func(context.Context) (contracts.HardwareInventory, error) {
			return contracts.HardwareInventory{
				OS: "darwin", Arch: "arm64", Hostname: "Mikes-Mac",
				CPU:          contracts.CPUInfo{Model: "Apple M3 Pro"},
				Memory:       contracts.MemoryInfo{TotalBytes: 36 << 30},
				Accelerators: []contracts.Accelerator{{Model: "Apple M3 Pro"}},
			}, nil
		},
		GetLog: func(_ context.Context, name string, _ int64) (logs.Content, error) {
			return logs.Content{Name: name, Content: `{"time":"t","level":"ERROR","msg":"bind failed on Mikes-Mac","error":"from 8.8.8.8"}`}, nil
		},
	})
	call := func(role auth.Role) diagnostics.Report {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/report", nil)
		r = r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{Person: auth.Person{ID: "p", Role: role}}))
		w := httptest.NewRecorder()
		srv.handleReport(w, r)
		var rep diagnostics.Report
		if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil || w.Code != http.StatusOK {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		return rep
	}
	member := call(auth.RoleMember)
	if member.OS != "darwin" || member.MemoryGB != 36 || len(member.GPUs) != 1 || member.Contract != contracts.ContractVersion {
		t.Fatalf("member report: %+v", member)
	}
	if len(member.Log) != 0 {
		t.Fatalf("a Member's report has the log: %q", member.Log)
	}
	admin := call(auth.RoleAdmin)
	if got := strings.Join(admin.Log, "\n"); got != `t ERROR bind failed on <computer> error="from <public IPv4>"` {
		t.Fatalf("admin log: %q", got)
	}
}
