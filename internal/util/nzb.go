package util

import (
	"net/url"
	"strings"
)

func HashNZBFileLink(link string) string {
	if u, err := url.Parse(link); err == nil {
		if strings.HasSuffix(strings.TrimSuffix(u.Path, "/"), "/api") {
			q := u.Query()
			t, id := q.Get("t"), q.Get("id")
			if (t == "get" || t == "g") && id != "" {
				for key := range q {
					if key != "t" && key != "id" {
						q.Del(key)
					}
				}
				u.RawQuery = q.Encode()
				return MD5Hash(u.String())
			}
		}
	}
	return MD5Hash(CleanNZBFileLink(link))
}

func CleanNZBFileLink(link string) string {
	link, _, ok := strings.Cut(link, "?")
	if !ok {
		link, _, _ = strings.Cut(link, "&")
	}
	link, _, _ = strings.Cut(link, "#")
	return link
}
