package app

import (
	"errors"
	"os"
	"strings"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The person's own Hugging Face token downloads picture and clip models
// behind a license, from the vendor, with their account (#551). It's kept
// in the secret store, never shown, and sent only to Hugging Face for those
// files.

const huggingFaceTokenName = "huggingface-token"

var errHuggingFaceToken = contracts.NewError("HUGGINGFACE_TOKEN_INVALID", nil, errors.New("that isn't a Hugging Face token; copy a read token from huggingface.co/settings/tokens, which starts with hf_"))

// setHuggingFaceToken keeps token, or forgets it when it's empty.
func (a *App) setHuggingFaceToken(token string) error {
	secrets := auth.NewSecretStore(a.Config.Get().DataDir)
	token = strings.TrimSpace(token)
	if token == "" {
		if err := secrets.Delete(huggingFaceTokenName); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		a.invalidateCapabilities()
		return nil
	}
	if !strings.HasPrefix(token, "hf_") || len(token) < 20 || len(token) > 200 || strings.ContainsAny(token, " \t\r\n") {
		return errHuggingFaceToken
	}
	if err := secrets.Write(huggingFaceTokenName, token); err != nil {
		return err
	}
	a.invalidateCapabilities()
	return nil
}

// huggingFaceToken is the token, or "".
func (a *App) huggingFaceToken() string {
	if a.Config == nil {
		return ""
	}
	token, _ := auth.NewSecretStore(a.Config.Get().DataDir).Read(huggingFaceTokenName)
	return strings.TrimSpace(token)
}
