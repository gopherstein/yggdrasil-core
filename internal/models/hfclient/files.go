package hfclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// File is a file in a Hugging Face repository, with its size and the SHA-256
// of its content.
type File struct {
	Path   string
	URL    string
	Size   uint64
	SHA256 string
}

// Repo is a model file and, for a vision model, the projector beside it.
type Repo struct {
	Model     File
	Projector *File
}

// projectorPrefs orders a repository's projectors: full precision first,
// since a quantized one sees less; it is a small file either way.
var projectorPrefs = []string{"f16", "bf16", "f32", "q8_0"}

// ParseResolveURL splits a https://huggingface.co/{repo}/resolve/{rev}/{path}
// URL. ok is false for any other address.
func ParseResolveURL(raw string) (repo, rev, file string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || (u.Host != "huggingface.co" && u.Host != "www.huggingface.co") {
		return "", "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 5)
	if len(parts) < 5 || parts[2] != "resolve" || parts[0] == "" || parts[1] == "" || parts[3] == "" || parts[4] == "" {
		return "", "", "", false
	}
	return parts[0] + "/" + parts[1], parts[3], parts[4], true
}

// Resolve reads the repository behind a model's download URL: the model
// file's size and SHA-256, and the projector a vision model needs (#191).
// The projector is chosen here, from the repository's own files, never from
// what a client asks for.
func (c *Client) Resolve(ctx context.Context, sourceURL string) (Repo, error) {
	repo, rev, file, ok := ParseResolveURL(sourceURL)
	if !ok {
		return Repo{}, fmt.Errorf("not a Hugging Face download address")
	}
	files, err := c.tree(ctx, repo, rev)
	if err != nil {
		return Repo{}, err
	}
	out := Repo{}
	found := false
	for _, f := range files {
		if f.Path == file {
			out.Model, found = f, true
		}
	}
	if !found {
		return Repo{}, fmt.Errorf("%s has no file %s", repo, file)
	}
	if p, ok := pickProjector(files, file); ok {
		out.Projector = &p
	}
	return out, nil
}

// pickProjector is the projector for model: one in the same folder, by
// projectorPrefs.
func pickProjector(files []File, model string) (File, bool) {
	dir := path.Dir(model)
	best, bestRank := File{}, -1
	for _, f := range files {
		low := strings.ToLower(path.Base(f.Path))
		if !strings.Contains(low, "mmproj") || !strings.HasSuffix(low, ".gguf") || path.Dir(f.Path) != dir {
			continue
		}
		rank := len(projectorPrefs)
		for i, p := range projectorPrefs {
			// "bf16" also contains "f16".
			if strings.Contains(strings.ReplaceAll(low, "bf16", "bf_16"), p) || (p == "bf16" && strings.Contains(low, "bf16")) {
				rank = i
				break
			}
		}
		if bestRank < 0 || rank < bestRank {
			best, bestRank = f, rank
		}
	}
	return best, bestRank >= 0
}

type treeEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		Oid string `json:"oid"`
	} `json:"lfs"`
}

// tree lists a repository's files at rev.
func (c *Client) tree(ctx context.Context, repo, rev string) ([]File, error) {
	u := fmt.Sprintf("%s/models/%s/tree/%s?recursive=true", c.BaseURL, repo, url.PathEscape(rev))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Toskar/0.1")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("huggingface api status %d", resp.StatusCode)
	}
	var entries []treeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	out := make([]File, 0, len(entries))
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		f := File{Path: e.Path, Size: uint64(max(e.Size, 0)),
			URL: fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s", repo, rev, e.Path)}
		if e.LFS != nil {
			f.SHA256 = e.LFS.Oid
		}
		out = append(out, f)
	}
	return out, nil
}
