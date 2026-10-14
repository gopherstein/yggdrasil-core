package api

import (
	"net/http"
	"os"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/diagnostics"
	"github.com/yeixio/toskar-core/internal/version"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// reportLogLines is how many of Core's last warnings and errors a report
// carries.
const reportLogLines = 40

// handleReport gathers what a "Report this" sends with a problem (#588):
// versions, the kind of computer, and for an Admin Core's last warnings
// and errors. Everything is scrubbed (diagnostics.Scrub) first, and the
// person sees all of it before it's sent.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	offer := version.CurrentOffer()
	host, _ := os.Hostname()
	rep := diagnostics.Report{
		Version:  offer.Version,
		Commit:   offer.Commit,
		Contract: contracts.ContractVersion,
	}
	if s.deps.Hardware != nil {
		if inv, err := s.deps.Hardware(r.Context()); err == nil {
			if inv.Hostname != "" {
				host = inv.Hostname
			}
			rep.OS, rep.Arch, rep.CPU = inv.OS, inv.Arch, inv.CPU.Model
			rep.MemoryGB = int((inv.Memory.TotalBytes + 1<<29) >> 30)
			for _, a := range inv.Accelerators {
				if name := a.Model; name != "" {
					rep.GPUs = append(rep.GPUs, name)
				}
			}
		}
	}
	if s.deps.GetLog != nil && auth.PrincipalFrom(r.Context()).Person.Role.AtLeast(auth.RoleAdmin) {
		if c, err := s.deps.GetLog(r.Context(), "daemon.log", 256<<10); err == nil {
			rep.Log = diagnostics.ReportLog(c.Content, host, reportLogLines)
		}
	}
	rep.CPU = diagnostics.Scrub(rep.CPU, host)
	writeJSON(w, http.StatusOK, rep)
}
