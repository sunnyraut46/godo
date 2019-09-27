package framework

import (
	"net/http"
	"regexp"
)

var Endpoints = map[*regexp.Regexp]string{
	regexp.MustCompile(`^/v2/account/keys$`):              "/v2/account/keys",
	regexp.MustCompile(`^/v2/account/keys/[^/]+$`):        "/v2/account/keys/{key_id}",
	regexp.MustCompile(`^/v2/regions$`):                   "/v2/regions",
	regexp.MustCompile(`^/v2/droplets$`):                  "/v2/droplets",
	regexp.MustCompile(`^/v2/droplets/\d+$`):              "/v2/droplets/{droplet_id}",
	regexp.MustCompile(`^/v2/droplets/\d+/actions$`):      "/v2/droplets/{droplet_id}/actions",
	regexp.MustCompile(`^/v2/droplets/\d+/actions/\d+$`):  "/v2/droplets/{droplet_id}/actions/{action_id}",
	regexp.MustCompile(`^/v2/load_balancers$`):            "/v2/load_balancers",
	regexp.MustCompile(`^/v2/load_balancers/[^/]+$`):      "/v2/load_balancers/{load_balancer_id}",
	regexp.MustCompile(`^/v2/tags$`):                      "/v2/tags",
	regexp.MustCompile(`^/v2/tags/[^/]+$`):                "/v2/tags/{tag}",
	regexp.MustCompile(`^/v2/kubernetes/clusters$`):       "/v2/kubernetes/clusters",
	regexp.MustCompile(`^/v2/kubernetes/clusters/[^/]+$`): "/v2/kubernetes/clusters/{cluster_id}",
}

func lookupEndpoint(req *http.Request) string {
	path := req.URL.EscapedPath()
	for re, endpoint := range Endpoints {
		if re.MatchString(path) {
			return endpoint
		}
	}
	return "unknown"
}
