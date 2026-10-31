package secrets

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		// Each pattern in sensitiveKeyPatterns
		{"PASSWORD", "DB_PASSWORD", true},
		{"PASS", "SMTP_PASS", true},
		{"PASSWD", "ROOT_PASSWD", true},
		{"PWD", "DB_PWD", true},
		{"SECRET", "JWT_SECRET", true},
		{"KEY", "SIGNING_KEY", true},
		{"TOKEN", "ACCESS_TOKEN", true},
		{"CREDENTIAL", "AWS_CREDENTIAL", true},
		{"CREDS", "GCP_CREDS", true},
		{"API_KEY", "STRIPE_API_KEY", true},
		{"APIKEY", "STRIPE_APIKEY", true},
		{"PRIVATE", "PRIVATE_KEY_PATH", true},
		{"AUTH", "AUTH_HEADER", true},
		{"CERT", "TLS_CERT_FILE", true},
		{"SSL_ prefix", "SSL_CERT_PATH", true},
		{"TLS_ prefix", "TLS_KEY_FILE", true},
		{"ENCRYPT", "ENCRYPT_ALGO", true},
		{"ACCESS_KEY", "AWS_ACCESS_KEY_ID", true},
		{"SESSION", "SESSION_SECRET", true},
		{"DSN", "SENTRY_DSN", true},
		{"CONNECTION_STRING", "AZURE_STORAGE_CONNECTION_STRING", true},
		{"CONNECTIONSTRING", "ConnectionStrings__Default", true},
		{"CONN_STR", "DB_CONN_STR", true},
		{"COOKIE", "RABBITMQ_ERLANG_COOKIE", true},
		{"SALT", "NONCE_SALT", true},
		{"WEBHOOK", "SLACK_WEBHOOK_URL", true},
		{"label key", "app.db.password", true},

		// Case insensitivity
		{"lowercase password", "db_password", true},
		{"mixed case secret", "Jwt_Secret", true},
		{"all lowercase token", "access_token", true},
		{"camelCase auth", "oAuthToken", true},

		// Non-sensitive keys
		{"PATH", "PATH", false},
		{"HOME", "HOME", false},
		{"PORT", "PORT", false},
		{"DATABASE_URL", "DATABASE_URL", false},
		{"LOG_LEVEL", "LOG_LEVEL", false},
		{"NODE_ENV", "NODE_ENV", false},
		{"HOSTNAME", "HOSTNAME", false},
		{"working directory", "PWD", false},
		{"previous working directory", "OLDPWD", false},
		{"OCI authors label", "org.opencontainers.image.authors", false},
		{"Passenger setting", "PASSENGER_APP_ENV", false},
		{"FTP passive mode", "FTP_PASSIVE", false},
		{"proxy passthrough", "traefik.tcp.routers.db.tls.passthrough", false},
		{"benign word next to a real one", "AUTHOR_TOKEN", true},
		{"Authorization header", "HTTP_AUTHORIZATION", true},
		{"Authorization header label", "traefik.http.middlewares.api.headers.customrequestheaders.Authorization", true},
		{"British spelling", "PROXY_AUTHORISATION", true},
		{"author label", "author", false},
		{"git author", "GIT_AUTHOR_NAME", false},
		{"empty key", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSensitiveKey(tc.key)
			if got != tc.want {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// maskValueCase is a MaskValue input and its expected result.
type maskValueCase struct{ name, in, want string }

// testMaskValue checks MaskValue on each case and, when stable, that masking
// the result again changes nothing: what is masked may be masked again on
// its way to the browser.
func testMaskValue(t *testing.T, cases []maskValueCase, stable bool) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskValue(tc.in)
			if got != tc.want {
				t.Errorf("MaskValue(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
			if !stable {
				return
			}
			if again := MaskValue(got); again != got {
				t.Errorf("MaskValue(%q) = %q, masked again %q", tc.in, got, again)
			}
		})
	}
}

func TestMaskValue(t *testing.T) {
	testMaskValue(t, []maskValueCase{
		{"plain value", "info", "info"},
		{"empty", "", ""},
		{"URL password", "postgres://shop:Pg-Secret-77@shop-db:5432/shop", "postgres://shop:********@shop-db:5432/shop"},
		{"URL password with @", "mysql://app:p@ss@db/app", "mysql://app:********@db/app"},
		{"URL empty user", "redis://:hunter2@cache:6379/0", "redis://:********@cache:6379/0"},
		{"URL token as user", "https://ghp_abc123@github.com/org/repo.git", "https://********@github.com/org/repo.git"},
		{"URL without userinfo", "https://example.com/a@b", "https://example.com/a@b"},
		{"URL query secret", "https://hooks.example/x?api_key=abc&page=2", "https://hooks.example/x?api_key=********&page=2"},
		{"URL with sensitive-looking query key", "postgres://app:pw@db/app?passfile=/x", "postgres://app:********@db/app?passfile=********"},
		{"several URLs", "redis://:a@h1,redis://:b@h2", "redis://:********@h1,redis://:********@h2"},
		{"mount options", "addr=10.0.0.5,username=app,password=pw", "addr=10.0.0.5,username=app,password=********"},
		{"pair", "PGPASSWORD=pw", "PGPASSWORD=********"},
		{"flag with value", "--requirepass=pw", "--requirepass=********"},
		{"shell command", "sh -c 'PGPASSWORD=pw psql --password hunter2 -h db'", "sh -c 'PGPASSWORD=******** psql --password ******** -h db'"},
		{"health output", "GET http://admin:pw@localhost/health -> 200 OK", "GET http://admin:********@localhost/health -> 200 OK"},
		{"whitespace kept", "a  b\tc", "a  b\tc"},
		// What follows a separator may still belong to a credential's value,
		// so the rest of the word is masked from it on.
		{"connection string", "Server=db;Database=shop;User Id=sa;Password=S3cret;", "Server=db;Database=shop;User Id=sa;Password=********"},
		{"ODBC password in a list", "Driver={pg};Server=db;Uid=sa;Pwd=S3cret", "Driver={pg};Server=db;Uid=sa;Pwd=********"},
		{"ODBC braced value", "Driver={pg};Server=db;Pwd={a;b9};Port=5432", "Driver={pg};Server=db;Pwd={********}"},
		{"ODBC braced value with spaces", "Server=db;Pwd={a b9}", "Server=db;Pwd={******** ********}"},
		{"separator inside a value", "addr=10.0.0.5,username=app,password=Pa;ss9", "addr=10.0.0.5,username=app,password=********"},
		{"what looks like an item inside a value", ",pass=;A=S3cret", ",pass=********"},
		{"brace that quotes no value", "a:{0,password=S3cret", "a:{0,password=********"},
		{"working directory outside a list", "PWD=/app", "PWD=/app"},
		{"working directory in a command chain", "export PWD=/srv;cd /srv", "export PWD=/srv;cd /srv"},
		{"working directory in a plain list", "PWD=/app,x=1", "PWD=/app,x=1"},
		{"brace outside a connection string", "--password {abc9 --port 1", "--password ******** --port 1"},
		{"uid and gid after a user flag", "docker run -u 1000:1000 app", "docker run -u 1000:1000 app"},
		{"command chain", "export A=1;PGPASSWORD=pw;psql", "export A=1;PGPASSWORD=********"},
		{"JSON", `{"user":"app","password":"S3cret"}`, `{"user":"app","password":"********"}`},
		{"JSON with spaces", `{ "token" : "abc", "page": "2" }`, `{ "token" : "********", "page": "2" }`},
		{"pretty JSON", `{"token": "abc", "page": 2}`, `{"token": "********", "page": 2}`},
		{"JSON after a quoted string looking like a member", `"":" {"password":"S3cret"}`, `"":" {"password":"********"}`},
		{"JSON after a sensitive string looking like a member", `"pass":" x {"token": "S3cret"}`, `"pass":"********"token": "********"}`},
		{"pair inside what looks like a JSON string", `"pass":" x PGPASSWORD="S3cret" y`, `"pass":"********"********" y`},
		{"flag inside what looks like a JSON string", `"pass":" -pass" S3cret`, `"pass":"********" ********`},
		{"JSON in a shell string", `curl -d "{\"password\":\"S3cret\"}" http://es`, `curl -d "{\"password\":\"********\"}" http://es`},
		{"header", "curl -H 'Authorization: Bearer abc123' http://api", "curl -H 'Authorization: Bearer ********' http://api"},
		{"header without scheme", "X-Api-Key: abc123", "X-Api-Key: ********"},
		{"other header", "Content-Type: application/json", "Content-Type: application/json"},
		{"key and value", "password: hunter2", "password: ********"},
		{"quoted value with spaces", "PGPASSWORD='pw 9' psql -h db", "PGPASSWORD='******** ********' psql -h db"},
		{"quoted flag value with spaces", `app --password "a b c" --verbose`, `app --password "******** ******** ********" --verbose`},
		{"quote closed inside the value", "PGPASSWORD='pw'x psql", "PGPASSWORD='********' psql"},
		{"user and password", `curl -u "elastic:S3cret" https://es:9200`, `curl -u "elastic:********" https://es:9200`},
		{"quoted user and password with spaces", `curl -u "elastic:pa ss9" https://es`, `curl -u "elastic:******** ********" https://es`},
		{"quoted user flag value with spaces", `curl --user="elastic:pa ss9" https://es`, `curl --user="elastic:******** ********" https://es`},
		{"URL after a user flag", "redis-cli -u redis://default:pw9@cache:6379", "redis-cli -u redis://default:********@cache:6379"},
		{"no user after a user flag", "date -u +%H:%M:%S", "date -u +%H:%M:%S"},
		{"user without password", "mysql -u root -h db", "mysql -u root -h db"},
		{"other value after a user flag", "app -u PGPASSWORD=pw", "app -u PGPASSWORD=********"},
		{"header after a user flag", "app -u Authorization: Bearer abc", "app -u Authorization: Bearer ********"},
	}, true)
}

// An Authorization value written as a pair ("Authorization=Bearer x", as in
// OTEL_EXPORTER_OTLP_HEADERS) holds a scheme and, after a space, the
// credentials.
func TestMaskValueAuthorizationPairs(t *testing.T) {
	testMaskValue(t, []maskValueCase{
		{"bearer", "Authorization=Bearer S3cret", "Authorization=******** ********"},
		{"api key scheme", "Authorization=ApiKey S3cret", "Authorization=******** ********"},
		{"api token scheme", "Authorization=Api-Token dt0c01.S3cret", "Authorization=******** ********"},
		{"unknown scheme", "Authorization=Splunk S3cret", "Authorization=******** ********"},
		{"in a header list", "x-tenant=a,Authorization=Bearer S3cret", "x-tenant=a,Authorization=******** ********"},
		{"lower case", "authorization=Basic dXNlcjpwYXNz", "authorization=******** ********"},
		{"flag", "--authorization=Bearer S3cret --verbose", "--authorization=******** ******** --verbose"},
		{"scheme under another sensitive key", "API_TOKEN=Bearer S3cret", "API_TOKEN=******** ********"},
		{"quoted, nothing after the quote masked", `AUTH="Bearer S3cret" next`, `AUTH="******** ********" next`},
		{"no scheme, next word kept", "DB_PASSWORD=S3cret next", "DB_PASSWORD=******** next"},
		{"connection string, next word kept", "Server=db;Password=Token;Port=1 next", "Server=db;Password=******** next"},
		{"header with an api key scheme", "Authorization: ApiKey S3cret", "Authorization: ApiKey ********"},
		{"header with an api token scheme", "Authorization:Api-Token S3cret", "Authorization:Api-Token ********"},
	}, false) // masked again, "Password=******** next" also masks "next": more, never less
}

func TestMaskEnvValueAndLabelsAuthorizationPairs(t *testing.T) {
	if got := MaskEnvValue("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Bearer S3cret"); strings.Contains(got, "S3cret") {
		t.Errorf("MaskEnvValue = %q", got)
	}
	got := MaskLabels(map[string]string{"com.example.otel": "x-tenant=a,Authorization=ApiKey S3cret"})
	if strings.Contains(got["com.example.otel"], "S3cret") {
		t.Errorf("MaskLabels = %q", got["com.example.otel"])
	}
}

// More ways credentials are written, and look-alikes left as they are.
func TestMaskValueMoreFormats(t *testing.T) {
	testMaskValue(t, []maskValueCase{
		{"header without a space", "Authorization:Bearer abc123", "Authorization:Bearer ********"},
		{"header value without a space", "X-Api-Key:abc123", "X-Api-Key:********"},
		{"key and value without a space", "password:hunter2", "password:********"},
		{"form", "user=app&password=S3cret", "user=app&password=********"},
		{"query without a scheme", "curl -f localhost:8080/health?token=S3cret", "curl -f localhost:8080/health?token=********"},
		{"URL password with a slash", "postgres://admin:pa/S3cret@db/app", "postgres://admin:********@db/app"},
		{"URL password with a hash", "mysql://admin:pa#S3cret@db/app", "mysql://admin:********@db/app"},
		{"URL password with a question mark", "mysql://admin:pa?S3cret@db/app", "mysql://admin:********@db/app"},
		{"Go MySQL DSN", "root:S3cret@tcp(db:3306)/app?parseTime=true", "root:********@tcp(db:3306)/app?parseTime=true"},
		{"Go MySQL DSN in a pair", "DB=root:S3cret@unix(/run/mysqld.sock)/app", "DB=root:********@unix(/run/mysqld.sock)/app"},
		{"Oracle JDBC URL", "jdbc:oracle:thin:scott/S3cret@db:1521:orcl", "jdbc:oracle:thin:scott/********@db:1521:orcl"},
		{"connection string password with a space", "Server=db;Password=pa S3cret;", "Server=db;Password=******** ********"},
		{"JSON number", `{"password": 1234567, "port": 5432}`, `{"password": "********", "port": 5432}`},
		{"XML element", "<db><password>S3cret</password><user>app</user></db>", "<db><password>********</password><user>app</user></db>"},
		{"XML element with spaces and attributes", `<password type="plain">pa S3cret</password>`, `<password type="plain">********</password>`},
		{"TOML", `password = "S3cret"`, `password = "********"`},
		{"TOML without a space after =", `password ="S3cret"`, `password ="********"`},
		{"NUL-separated environment", "/usr/bin\x00DG_PASSWORD=S3cret\x00HOME=/root", "/usr/bin\x00DG_PASSWORD=********\x00HOME=/root"},
		{"flag holding a pair", "docker run --env=DB_PASSWORD=S3cret img", "docker run --env=DB_PASSWORD=******** img"},
		{"flag holding a dotted pair", "helm install x --set=auth.password=S3cret", "helm install x --set=auth.password=********"},
		{"no-break spaces", "app --password S3cret", "app --password ********"},
		{"no-break space inside a value", "--password pa ss", "--password ******** ********"},

		{"image tag", "keycloak:22 token-service:latest redis:7-alpine", "keycloak:22 token-service:latest redis:7-alpine"},
		{"port and @ in a URL path", "https://registry.npmjs.org:443/@scope/pkg", "https://registry.npmjs.org:443/@scope/pkg"},
		{"package with a version", "lodash/fp@4.17", "lodash/fp@4.17"},
		{"XML element of no credential", "<name>web</name>", "<name>web</name>"},
		{"TOML of no credential", `name = "web"`, `name = "web"`},
		{"flag holding a pair of no credential", "--env=PATH=/usr/bin", "--env=PATH=/usr/bin"},
		{"no-break space of no credential", "a b", "a b"},
		{"query of no credential", "/health?page=2&size=10", "/health?page=2&size=10"},
	}, true)
}

func TestMaskCommandArgs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want []string
	}{
		{"nil", nil, nil},
		{"flag then value", []string{"redis-server", "--requirepass", "pw", "--port", "6379"}, []string{"redis-server", "--requirepass", Masked, "--port", "6379"}},
		{"flag=value", []string{"app", "--db-password=pw", "--verbose"}, []string{"app", "--db-password=" + Masked, "--verbose"}},
		{"boolean sensitive flag", []string{"mongod", "--auth", "--bind_ip_all"}, []string{"mongod", "--auth", "--bind_ip_all"}},
		{"one-letter flag left alone", []string{"psql", "-p", "5432"}, []string{"psql", "-p", "5432"}},
		{"java property", []string{"java", "-Djavax.net.ssl.trustStorePassword=pw", "-jar", "app.jar"}, []string{"java", "-Djavax.net.ssl.trustStorePassword=" + Masked, "-jar", "app.jar"}},
		{"URL argument", []string{"migrate", "-database", "postgres://u:pw@db/app"}, []string{"migrate", "-database", "postgres://u:" + Masked + "@db/app"}},
		{"shell form", []string{"sh", "-c", "exec app --token abc"}, []string{"sh", "-c", "exec app --token " + Masked}},
		{"user and password", []string{"curl", "-u", "elastic:S3cret", "https://es:9200"}, []string{"curl", "-u", "elastic:" + Masked, "https://es:9200"}},
		{"header argument", []string{"curl", "-H", "Authorization: Basic dXNlcjpwYXNz", "http://api"}, []string{"curl", "-H", "Authorization: Basic " + Masked, "http://api"}},
		// Quotes inside an exec-form argument are literal characters.
		{"a quote never spans arguments", []string{"app", "--password", "'a", "--verbose", "x"}, []string{"app", "--password", "'" + Masked, "--verbose", "x"}},
		// An exec-form argument reaches the program whole, so a value in it
		// goes on to its end, spaces included.
		{"value with spaces", []string{"app", "--passphrase", "correct horse battery"}, []string{"app", "--passphrase", Masked + " " + Masked + " " + Masked}},
		{"flag=value with spaces", []string{"app", "--passphrase=correct horse battery"}, []string{"app", "--passphrase=" + Masked + " " + Masked + " " + Masked}},
		{"pair with spaces", []string{"env", "DB_PASSWORD=correct horse battery", "app"}, []string{"env", "DB_PASSWORD=" + Masked + " " + Masked + " " + Masked, "app"}},
		{"user and password with spaces", []string{"curl", "-u", "elastic:correct horse", "https://es"}, []string{"curl", "-u", "elastic:" + Masked + " " + Masked, "https://es"}},
		{"header value with spaces", []string{"curl", "-H", `Authorization: Digest username="app", response="6629fae4"`, "http://api"}, []string{"curl", "-H", `Authorization: Digest ******** ********="********"`, "http://api"}},
		{"JSON member holding the value", []string{"app", "--config", `{"credentials": "AKIAEXAMPLE1 --password=x more"}`}, []string{"app", "--config", `{"credentials": "********"}`}},
		{"flag after the value", []string{"app", "--db-password=x --token", "S3cret"}, []string{"app", "--db-password=" + Masked + " --" + Masked, Masked}},
		{"dash after the value", []string{"app", `-"token":" abc" -"x`}, []string{"app", `-"token":"********" -"********`}},
		// The command a shell runs is shell text, where a space ends a value.
		{"shell form with combined options", []string{"bash", "-lc", "PGPASSWORD=pw psql -h db"}, []string{"bash", "-lc", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"arguments after a shell command", []string{"sh", "-c", `exec app "$@"`, "app", "--token", "a b"}, []string{"sh", "-c", `exec app "$@"`, "app", "--token", Masked + " " + Masked}},
		{"shell given by its path", []string{"/bin/sh", "-ec", "PGPASSWORD=pw psql -h db"}, []string{"/bin/sh", "-ec", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"shell run by busybox", []string{"busybox", "sh", "-c", "PGPASSWORD=pw psql -h db"}, []string{"busybox", "sh", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"shell options before -c", []string{"bash", "-e", "-c", "PGPASSWORD=pw psql -h db"}, []string{"bash", "-e", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"shell option without a value", []string{"bash", "--norc", "-c", "PGPASSWORD=pw psql -h db"}, []string{"bash", "--norc", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"shell option with a value", []string{"/bin/bash", "-o", "pipefail", "-c", "PGPASSWORD=pw pg_dump -h db"}, []string{"/bin/bash", "-o", "pipefail", "-c", "PGPASSWORD=" + Masked + " pg_dump -h db"}},
		{"shell option with a value, sh", []string{"sh", "-o", "pipefail", "-c", "PGPASSWORD=pw psql -h db"}, []string{"sh", "-o", "pipefail", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"combined shell options with a value", []string{"bash", "-euo", "pipefail", "-c", "PGPASSWORD=pw psql -h db"}, []string{"bash", "-euo", "pipefail", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		{"shell option turned off with +", []string{"bash", "+O", "extglob", "-c", "PGPASSWORD=pw psql -h db"}, []string{"bash", "+O", "extglob", "-c", "PGPASSWORD=" + Masked + " psql -h db"}},
		// Only a shell takes a command after -c: postgres takes a setting.
		{"-c of a program other than a shell", []string{"postgres", "-c", "password=correct horse battery"}, []string{"postgres", "-c", "password=" + Masked + " " + Masked + " " + Masked}},
		{"-c of an unknown program", []string{"app", "-c", "DB_PASSWORD=correct horse battery"}, []string{"app", "-c", "DB_PASSWORD=" + Masked + " " + Masked + " " + Masked}},
		{"shell name as an option's value", []string{"app", "--password", "sh", "-c", "PGPASSWORD=pw psql"}, []string{"app", "--password", Masked, "-c", "PGPASSWORD=" + Masked + " " + Masked}},
		{"shell name in a masked argument", []string{"curl", "-H", "Authorization: /bin/sh", "-ec", "PGPASSWORD=pw psql"}, []string{"curl", "-H", "Authorization: " + Masked, "-ec", "PGPASSWORD=" + Masked + " " + Masked}},
		{"flag holding a pair", []string{"docker", "run", "--env=DB_PASSWORD=S3cret", "img"}, []string{"docker", "run", "--env=DB_PASSWORD=" + Masked, "img"}},
		{"header without a space", []string{"curl", "-H", "Authorization:Bearer S3cret", "http://api"}, []string{"curl", "-H", "Authorization:Bearer " + Masked, "http://api"}},
		{"no-break spaces", []string{"app --password S3cret"}, []string{"app --password " + Masked}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := MaskCommand(nil, tc.in)
			if !slices.Equal(got, tc.want) {
				t.Errorf("MaskCommand(nil, %q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
			// What is masked may be masked again on its way to the browser.
			if _, again := MaskCommand(nil, got); !slices.Equal(again, got) {
				t.Errorf("MaskCommand(nil, %q) = %q, masked again %q", tc.in, got, again)
			}
		})
	}
}

// Docker runs the entrypoint with the command appended, so a flag ending the
// entrypoint takes its value from the command.
func TestMaskCommand(t *testing.T) {
	entrypoint, cmd := MaskCommand([]string{"redis-server", "--requirepass"}, []string{"S3cret", "--port", "6379"})
	if want := []string{"redis-server", "--requirepass"}; !slices.Equal(entrypoint, want) {
		t.Errorf("entrypoint = %q, want %q", entrypoint, want)
	}
	if want := []string{Masked, "--port", "6379"}; !slices.Equal(cmd, want) {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}

	entrypoint, cmd = MaskCommand(nil, []string{"app"})
	if entrypoint != nil || !slices.Equal(cmd, []string{"app"}) {
		t.Errorf("got %q, %q; want nil entrypoint kept", entrypoint, cmd)
	}

	// The command a shell in the entrypoint runs is shell text.
	_, cmd = MaskCommand([]string{"sh", "-c"}, []string{"exec app --token abc --verbose"})
	if want := []string{"exec app --token " + Masked + " --verbose"}; !slices.Equal(cmd, want) {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

func TestMaskEnvValue(t *testing.T) {
	if got := MaskEnvValue("DB_PASSWORD", "pw"); got != Masked {
		t.Errorf("sensitive key: got %q", got)
	}
	if got := MaskEnvValue("DATABASE_URL", "postgres://admin:S3cret@db/app"); got != "postgres://admin:"+Masked+"@db/app" {
		t.Errorf("URL value: got %q", got)
	}
	if got := MaskEnvValue("PWD", "/app"); got != "/app" {
		t.Errorf("working directory: got %q", got)
	}
	// Whether a password is empty or authentication off is masked too.
	for _, kv := range [][2]string{{"MYSQL_ROOT_PASSWORD", ""}, {"AUTH_ENABLED", "false"}} {
		if got := MaskEnvValue(kv[0], kv[1]); got != Masked {
			t.Errorf("%s=%q: got %q, want it masked", kv[0], kv[1], got)
		}
	}
	// Benign words don't make a label sensitive.
	if got := MaskEnvValue("org.opencontainers.image.authors", "Jane Doe"); got != "Jane Doe" {
		t.Errorf("authors label: got %q", got)
	}
}

func TestMaskLabels(t *testing.T) {
	if MaskLabels(nil) != nil {
		t.Error("nil labels should stay nil")
	}
	in := map[string]string{
		"traefik.http.middlewares.auth.basicauth.users": "admin:$apr1$x",
		"app.db.password":            "LabelPass1",
		"app.upstream":               "http://u:pw@backend",
		"com.docker.compose.project": "shop",
	}
	got := MaskLabels(in)
	want := map[string]string{
		"traefik.http.middlewares.auth.basicauth.users": Masked,
		"app.db.password":            Masked,
		"app.upstream":               "http://u:" + Masked + "@backend",
		"com.docker.compose.project": "shop",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if in["app.db.password"] != "LabelPass1" {
		t.Error("MaskLabels must not modify its input")
	}
}

// Container owners choose the values masked, so the time masking takes must
// grow linearly with their length: four times the input takes about four
// times as long, where a quadratic path takes sixteen. The scan is timed on
// values longer than the ones MaskValue scans: shorter ones take too little
// time to compare, and regexp may match a short input with a faster engine
// than a long one (for inputs up to 256 Kbit divided by the pattern's size),
// which would change the time per byte between the two lengths.
func TestMaskTimeIsLinear(t *testing.T) {
	for _, tc := range []struct {
		name, unit, tail string
	}{
		{"list items without a value", "a,", "="},
		{"command chain without a value", "a;", "=x"},
		{"JSON key of escaped quotes", `\"`, `":"v"`},
		{"pairs in pairs", "a=", "x"},
		{"JSON keys before a number", `"a":`, "1"},
		{"URLs without a password", "x://u:p/", "@h"},
		{"XML tags", "<a>", "</b>"},
		{"keys and colons", "a:", "x"},
		{"no-break spaces", "a\u00a0", "--password\u00a0x"},
		{"Go DSN users", "a:", "@tcp("},
	} {
		t.Run(tc.name, func(t *testing.T) {
			short := repeatTo(tc.unit, tc.tail, 2*maxValueLen)
			long := repeatTo(tc.unit, tc.tail, 8*maxValueLen)
			// The fastest of a few runs is the least disturbed. The runs
			// alternate, so a slowdown of the machine falls on both lengths.
			shortTime, longTime := time.Duration(1<<63-1), time.Duration(1<<63-1)
			for range 3 {
				shortTime = min(shortTime, scanTime(short))
				longTime = min(longTime, scanTime(long))
			}
			if longTime > 10*shortTime {
				t.Errorf("%d bytes took %v, %d bytes %v", len(short), shortTime, len(long), longTime)
			}
		})
	}
}

// repeatTo returns about n bytes: unit repeated, then tail.
func repeatTo(unit, tail string, n int) string {
	return strings.Repeat(unit, (n-len(tail))/len(unit)) + tail
}

// scanTime returns how long masking s takes, as free text and as a command
// argument.
func scanTime(s string) time.Duration {
	start := time.Now()
	new(masker).text(s, false)
	new(masker).text(s, true)
	return time.Since(start)
}

// A value too long to be configuration is masked whole rather than scanned.
func TestMaskValueTooLong(t *testing.T) {
	longest := strings.Repeat("a", maxValueLen)
	if got := MaskValue(longest); got != longest {
		t.Errorf("value of %d bytes changed", len(longest))
	}
	if got := MaskValue(longest + "a"); got != Masked {
		t.Errorf("value of %d bytes: got %d bytes, want it masked", len(longest)+1, len(got))
	}
	// What the long argument announces is unknown, and it decides how the
	// arguments after it read ("Authorization:" makes "Bearer" a scheme and
	// the one after it the token), so they are all masked.
	for _, end := range []string{"a", " --password", " -u", " Authorization:", " -o"} {
		for _, next := range [][]string{
			{"--port", "80"},
			{"S3cret"},
			{"admin:S3cret"},
			{"Bearer", "S3cret"},
			{"Authorization:", "S3cret"},
		} {
			in := append([]string{"app", longest + end}, next...)
			want := append([]string{"app"}, slices.Repeat([]string{Masked}, len(in)-1)...)
			if _, got := MaskCommand(nil, in); !slices.Equal(got, want) {
				t.Errorf("after an argument of %d bytes ending in %q: got %.40q, want the rest masked", len(in[1]), end, got)
			}
		}
	}
}
