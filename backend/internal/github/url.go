package github

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

var ErrInvalidURL = errors.New("invalid_url")

func ParsePRURL(raw string) (owner, repo string, number int, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, ErrInvalidURL
	}

	if u.Scheme != "https" || u.Host != "github.com" {
		return "", "", 0, ErrInvalidURL
	}

	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimSuffix(path, ".diff")
	parts := strings.Split(path, "/")

	if len(parts) != 5 {
		return "", "", 0, ErrInvalidURL
	}

	if parts[0] != "" || parts[1] == "" || parts[2] == "" || parts[3] != "pull" {
		return "", "", 0, ErrInvalidURL
	}

	for _, digit := range parts[4] {
		if digit < '0' || digit > '9' {
			return "", "", 0, ErrInvalidURL
		}
	}

	number, err = strconv.Atoi(parts[4])
	if err != nil || number <= 0 {
		return "", "", 0, ErrInvalidURL
	}

	return parts[1], parts[2], number, nil
}
