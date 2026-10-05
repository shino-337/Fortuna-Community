package poddetail

import "testing"

func TestRedactCommandLine(t *testing.T) {
	cases := map[string]string{
		"nginx -g daemon off;":                                  "nginx -g daemon off;",
		"app --password=hunter2 --port=8080":                    "app --password=[REDACTED] --port=8080",
		"app --api-key abc123 --verbose":                        "app --api-key [REDACTED] --verbose",
		"app --token=t1 --client-secret s2":                     "app --token=[REDACTED] --client-secret [REDACTED]",
		"env DB_PASSWORD=pw API_TOKEN=tk ./run":                 "env DB_PASSWORD=[REDACTED] API_TOKEN=[REDACTED] ./run",
		"worker postgres://fortuna:s3cret@db:5432/app":          "worker postgres://fortuna:[REDACTED]@db:5432/app",
		"redis-server --requirepass p --auth-file /etc/a --tls": "redis-server --requirepass [REDACTED] --auth-file [REDACTED] --tls",
		"app --passwordless --tls":                              "app --passwordless --tls",
		"/usr/bin/token-refresher --interval=30s":               "/usr/bin/token-refresher --interval=30s",
		"": "",
	}
	for in, want := range cases {
		if got := redactCommandLine(in); got != want {
			t.Errorf("redactCommandLine(%q) = %q, want %q", in, got, want)
		}
	}
}
