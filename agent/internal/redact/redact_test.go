package redact

import "testing"

func TestCommandLine(t *testing.T) {
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
		// Values glued to a flag or carried in headers, JSON and form bodies.
		"mysql -uroot -pS3cret db":                    "mysql -uroot -p[REDACTED] db",
		"/usr/bin/mysqldump -pS3cret --all-databases": "/usr/bin/mysqldump -p[REDACTED] --all-databases",
		"ls -pa":                            "ls -pa",
		"curl -u admin:hunter2 https://api": "curl -u admin:[REDACTED] https://api",
		`curl -H "Authorization: Bearer eyJhbGciOi.payload.sig" https://x`: `curl -H "Authorization: [REDACTED]" https://x`,
		`curl -d {"username":"admin","password":"Adm1n!pass"} http://core`: `curl -d {"username":"admin","password":"[REDACTED]"} http://core`,
		"curl http://h/login?user=a&password=pw&x=1":                       "curl http://h/login?user=a&password=[REDACTED]&x=1",
	}
	for in, want := range cases {
		if got := CommandLine(in); got != want {
			t.Errorf("CommandLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestText(t *testing.T) {
	cases := map[string]string{
		"Shell spawned in container (user=root command=bash -c id)": "Shell spawned in container (user=root command=bash -c id)",
		`output with \"token\":\"abc123\" inside`:                   `output with \"token\":\"[REDACTED]\" inside`,
		"connect to https://svc:pw@db.internal:5432":                "connect to https://svc:[REDACTED]@db.internal:5432",
		"header x-api-key: abcdef0123 sent":                         "header x-api-key: [REDACTED]",
		"Bearer abcdefghijklmnop seen":                              "Bearer [REDACTED] seen",
		"Rule --password flag usage detected":                       "Rule --password flag usage detected",
	}
	for in, want := range cases {
		if got := Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
}
