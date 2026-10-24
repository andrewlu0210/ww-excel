// Package update checks GitHub Releases for a newer version.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Release struct {
	// Version is the tag without its leading "v".
	Version string
	URL     string
	Notes   string
}

// API is the GitHub API root; injectable for tests.
var API = "https://api.github.com"

// ErrNoRelease means the repository has no published release yet (or is
// private: GitHub answers 404 for both).
var ErrNoRelease = errors.New("no published release")

// Latest returns the newest published (non-draft, non-prerelease)
// release of repo ("owner/name").
func Latest(ctx context.Context, client *http.Client, repo string) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if res.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub: %s", res.Status)
	}
	var body struct {
		Tag  string `json:"tag_name"`
		URL  string `json:"html_url"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	return Release{Version: strings.TrimPrefix(body.Tag, "v"), URL: body.URL, Notes: body.Body}, nil
}

// Newer reports whether version a is newer than b. Versions are dotted
// numbers ("1.2.10"); a pre-release suffix ("1.1.0-beta.2") sorts before
// the release itself.
func Newer(a, b string) bool {
	return compare(a, b) > 0
}

func compare(a, b string) int {
	an, apre := split(a)
	bn, bpre := split(b)
	for i := 0; i < max(len(an), len(bn)); i++ {
		x, y := at(an, i), at(bn, i)
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	case apre > bpre:
		return 1
	}
	return -1
}

func split(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	var nums []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(p)
		nums = append(nums, n)
	}
	return nums, pre
}

func at(n []int, i int) int {
	if i < len(n) {
		return n[i]
	}
	return 0
}
