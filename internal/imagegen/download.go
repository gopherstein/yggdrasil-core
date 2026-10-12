package imagegen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// download fetches url into path, resuming a partial file, and checks its
// size and checksum. progress is called with the bytes this call has added.
func download(ctx context.Context, client *http.Client, url, path string, size int64, sum string, progress func(int64)) error {
	return downloadAs(ctx, client, url, "", path, size, sum, progress)
}

// errNoAccess is a gated file the token can't get: none was given, or its
// account hasn't accepted the model's license (#551).
var errNoAccess = errors.New("no access")

// downloadAs is download with the person's Hugging Face token, for a file
// behind a license (#551). The token goes only to the address asked for;
// Go drops it when the download is redirected to another host.
func downloadAs(ctx context.Context, client *http.Client, url, token, path string, size int64, sum string, progress func(int64)) error {
	if done, _ := os.Stat(path); done != nil && done.Size() == size {
		if ok, err := checksum(path, sum); err == nil && ok {
			progress(size)
			return nil
		}
		_ = os.Remove(path)
	}
	part := path + ".part"
	var offset int64
	if st, err := os.Stat(part); err == nil && st.Size() < size {
		offset = st.Size()
	} else {
		_ = os.Remove(part)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "toskar-daemon")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		offset = 0
	case http.StatusUnauthorized, http.StatusForbidden:
		return errNoAccess
	default:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if offset > 0 {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	progress(offset)
	_, err = io.Copy(f, &progressReader{r: io.LimitReader(resp.Body, size-offset+1), fn: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	st, err := os.Stat(part)
	if err != nil {
		return err
	}
	if st.Size() != size {
		_ = os.Remove(part)
		return fmt.Errorf("expected %d bytes, got %d", size, st.Size())
	}
	ok, err := checksum(part, sum)
	if err != nil {
		return err
	}
	if !ok {
		_ = os.Remove(part)
		return fmt.Errorf("the download did not match its checksum")
	}
	return os.Rename(part, path)
}

func checksum(path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}

type progressReader struct {
	r  io.Reader
	fn func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.fn(int64(n))
	}
	return n, err
}
