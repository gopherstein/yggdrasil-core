package app

import (
	"crypto/tls"

	"github.com/yeixio/toskar-core/internal/api"
	"github.com/yeixio/toskar-core/internal/auth"
)

// setAPITLS has the API answer HTTPS (#213): with the person's own
// certificate when api_tls_cert and api_tls_key are set and can be read,
// and otherwise with the one Toskar makes and keeps.
func (a *App) setAPITLS() {
	cfg := a.Config.Get()
	info := api.APITLS{}
	var cert tls.Certificate
	var err error
	if cfg.APITLSCert != "" || cfg.APITLSKey != "" {
		if cert, err = auth.LoadCertificate(cfg.APITLSCert, cfg.APITLSKey); err == nil {
			info.Custom = true
		} else {
			info.Error = err.Error()
			a.Logger.Warn("own API certificate not used", "error", err)
		}
	}
	if !info.Custom {
		if cert, err = auth.APICertificate(auth.NewSecretStore(cfg.DataDir)); err != nil {
			a.Logger.Warn("API serves plain HTTP only: no certificate", "error", err)
			return
		}
	}
	info.Enabled = true
	info.Fingerprint = auth.CertFingerprint(cert)
	info.Short = auth.ShortFingerprint(info.Fingerprint)
	if cert.Leaf != nil {
		t := cert.Leaf.NotAfter
		info.NotAfter = &t
	}
	a.API.SetTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, info)
}
