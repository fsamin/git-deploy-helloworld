// git-deploy-helloworld is the demo application of git-deploy-operator. Its
// one page showcases everything the operator injects: environment variables,
// mounted configuration files, and the add-on connection URLs (DATABASE_URL,
// REDIS_URL) — and it logs enough to make `git-deploy logs -f` interesting.
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// version is bumped on each demo commit to make rolling updates visible.
const version = "v11"

// configPath is where the demo expects a mounted file:
// git-deploy file set /etc/app/config.yaml --from ./config.yaml
const configPath = "/etc/app/config.yaml"

func main() {
	log.Printf("git-deploy-helloworld %s starting on :8080", version)
	logStartup()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		greeting, cfg := readConfig()
		hostname, _ := os.Hostname()
		fmt.Fprintf(w, "%s from git-deploy-helloworld %s (pod %s)\n", greeting, version, hostname)

		fmt.Fprintf(w, "\n== Environment variables ==\n")
		for _, line := range envLines() {
			fmt.Fprintln(w, line)
		}

		fmt.Fprintf(w, "\n== Config file (%s) ==\n", configPath)
		if cfg == "" {
			fmt.Fprintf(w, "(not mounted — try: git-deploy file set %s --from ./config.yaml)\n", configPath)
		} else {
			fmt.Fprint(w, cfg)
			if !strings.HasSuffix(cfg, "\n") {
				fmt.Fprintln(w)
			}
		}

		fmt.Fprintf(w, "\n== PostgreSQL add-on (DATABASE_URL) ==\n")
		fmt.Fprintln(w, postgresStatus())

		fmt.Fprintf(w, "\n== Redis add-on (REDIS_URL) ==\n")
		fmt.Fprintln(w, redisStatus())

		log.Printf("%s %s from %s (%s)", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start).Round(time.Millisecond))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// logStartup says which injected pieces the app can see, so the very first
// lines of `git-deploy logs` tell the story.
func logStartup() {
	if _, err := os.Stat(configPath); err == nil {
		log.Printf("config file %s is mounted", configPath)
	} else {
		log.Printf("no config file at %s", configPath)
	}
	for _, name := range []string{"DATABASE_URL", "REDIS_URL"} {
		if os.Getenv(name) != "" {
			log.Printf("%s is set (%s)", name, maskValue(name, os.Getenv(name)))
		} else {
			log.Printf("%s is not set", name)
		}
	}
}

// readConfig returns the greeting and the raw content of the mounted file.
// A `greeting:` line in it changes the headline — editing the file from the
// CLI or the UI visibly changes the page.
func readConfig() (greeting, content string) {
	greeting = "Hello"
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return greeting, ""
	}
	content = string(raw)
	for _, line := range strings.Split(content, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "greeting:"); ok {
			if v = strings.TrimSpace(v); v != "" {
				greeting = v
			}
		}
	}
	return greeting, content
}

// serviceLinkVar matches the legacy Docker-links variables kubelet injects for
// every Service in the namespace (FOO_SERVICE_HOST, FOO_PORT_80_TCP_ADDR…):
// pure noise on this page, where the point is what *the operator* injected.
var serviceLinkVar = regexp.MustCompile(`(_SERVICE_(HOST|PORT)|_PORT($|_\d+_(TCP|UDP)))`)

// envLines renders the environment, sorted, with secrets masked and the
// Kubernetes machinery variables skipped.
func envLines() []string {
	var lines []string
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "KUBERNETES_") || serviceLinkVar.MatchString(name) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%s", name, maskValue(name, value)))
	}
	sort.Strings(lines)
	return lines
}

var (
	sensitiveName = regexp.MustCompile(`(?i)(secret|password|token|_key|apikey)`)
	urlPassword   = regexp.MustCompile(`(://[^:/@]+:)[^@]+@`)
)

// maskValue hides what should not appear on a public page: values of
// sensitive-looking variables entirely, passwords inside connection URLs.
func maskValue(name, value string) string {
	if sensitiveName.MatchString(name) {
		return "******** (masked)"
	}
	return urlPassword.ReplaceAllString(value, "$1***@")
}

// db is opened once: sql.DB pools and reconnects by itself afterwards.
var (
	dbOnce sync.Once
	db     *sql.DB
	dbErr  error
)

// postgresStatus increments and reports a visit counter in the add-on
// database — proof the injected DATABASE_URL actually reaches PostgreSQL.
func postgresStatus() string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "(not attached — try: git-deploy addon add postgresql)"
	}
	dbOnce.Do(func() {
		db, dbErr = sql.Open("postgres", dsn)
		if dbErr == nil {
			db.SetConnMaxLifetime(time.Minute)
			_, dbErr = db.Exec(`CREATE TABLE IF NOT EXISTS visits (id int PRIMARY KEY, count bigint NOT NULL)`)
		}
		if dbErr != nil {
			log.Printf("postgres setup failed: %v", dbErr)
		}
	})
	if dbErr != nil {
		return fmt.Sprintf("error: %v", dbErr)
	}
	var count int64
	err := db.QueryRow(`INSERT INTO visits (id, count) VALUES (1, 1)
		ON CONFLICT (id) DO UPDATE SET count = visits.count + 1
		RETURNING count`).Scan(&count)
	if err != nil {
		log.Printf("postgres query failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	return fmt.Sprintf("connected — %d visits recorded in the database", count)
}

// redisStatus increments a counter through a minimal RESP exchange, keeping
// the demo free of a redis client dependency.
func redisStatus() string {
	rawURL := os.Getenv("REDIS_URL")
	if rawURL == "" {
		return "(not attached — try: git-deploy addon add redis)"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		log.Printf("redis dial failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintf(conn, "*2\r\n$4\r\nINCR\r\n$4\r\nhits\r\n"); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	reply = strings.TrimSpace(reply)
	if !strings.HasPrefix(reply, ":") {
		return fmt.Sprintf("unexpected reply %q", reply)
	}
	return fmt.Sprintf("connected — %s hits counted in redis", strings.TrimPrefix(reply, ":"))
}
