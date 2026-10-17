package github

import (
	"net/url"
	"strings"
)

func validatePageURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ErrGitHubUnavailable
	}

	if u.Scheme != "https" ||
		u.Host != "api.github.com" ||
		u.User != nil {
		return ErrGitHubUnavailable
	}

	return nil
}

func nextPageURL(link string) string {
	for _, entry := range strings.Split(link, ",") {
		parts := strings.Split(entry, ";")
		if len(parts) < 2 {
			continue
		}

		for _, param := range parts[1:] {
			if strings.TrimSpace(param) != `rel="next"` {
				continue
			}

			target := strings.TrimSpace(parts[0])
			if strings.HasPrefix(target, "<") &&
				strings.HasSuffix(target, ">") {
				return target[1 : len(target)-1]
			}
		}
	}

	return ""
}
