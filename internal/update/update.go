package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const Repo = "mewisme/vutils"

type Result struct {
	Current string
	Latest  string
	URL     string
	Newer   bool
}

type releaseAPI struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check hits GitHub latest release for Repo and compares to current.
func Check(current string) (Result, error) {
	return CheckRepo(Repo, current)
}

// CheckRepo is like Check but for a custom owner/repo.
func CheckRepo(repo, current string) (Result, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vutils-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Result{}, fmt.Errorf("no releases found for %s", repo)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("github API: %s", resp.Status)
	}

	var rel releaseAPI
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Result{}, err
	}
	if rel.TagName == "" {
		return Result{}, fmt.Errorf("empty release tag")
	}

	cur := normalize(current)
	lat := normalize(rel.TagName)
	return Result{
		Current: cur,
		Latest:  lat,
		URL:     rel.HTMLURL,
		Newer:   compare(lat, cur) > 0,
	}, nil
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "dev") {
		return "0.0.0"
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// Drop pre-release / build metadata for compare.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// compare returns 1 if a>b, -1 if a<b, 0 if equal (major.minor.patch).
func compare(a, b string) int {
	ap := parse(a)
	bp := parse(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func parse(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
