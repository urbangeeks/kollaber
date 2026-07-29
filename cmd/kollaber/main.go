package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// defaultAPI points at the hosted service so a fresh install works out of the
// box for SaaS users. Self-hosted/local-dev users override via --api or
// KOLLABER_API (which is then saved to ~/.kollaber/config.json on login).
const defaultAPI = "https://kollaber.io"

// --- config ---

type config struct {
	Token  string `json:"token"`
	APIURL string `json:"api_url"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kollaber", "config.json")
}

func loadConfig() config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return config{APIURL: defaultAPI}
	}
	var c config
	_ = json.Unmarshal(data, &c)
	if c.APIURL == "" {
		c.APIURL = defaultAPI
	}
	return c
}

func saveConfig(c config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, data, 0600)
}

// --- HTTP helpers ---

func apiURL() string {
	if u := os.Getenv("KOLLABER_API"); u != "" {
		return u
	}
	return loadConfig().APIURL
}

func do(method, path string, body any) (*http.Response, error) {
	cfg := loadConfig()
	url := apiURL() + path

	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	return http.DefaultClient.Do(req)
}

func decodeOK(res *http.Response, dst any) error {
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(dst)
}

// --- environment lookup ---

type environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
}

func findEnv(name string) (environment, error) {
	res, err := do("GET", "/environments", nil)
	if err != nil {
		return environment{}, err
	}
	var envs []environment
	if err := decodeOK(res, &envs); err != nil {
		return environment{}, err
	}
	for _, e := range envs {
		if e.Name == name || e.ID == name {
			return e, nil
		}
	}
	return environment{}, fmt.Errorf("environment %q not found", name)
}

// --- commands ---

var version = "dev" // overridden at build time via -ldflags "-X main.version=..."

var rootCmd = &cobra.Command{
	Use:     "kollaber",
	Short:   "Kollaber CLI — infrastructure event timeline",
	Version: version,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and save a token",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiFlag, _ := cmd.Flags().GetString("api")
		token, _ := cmd.Flags().GetString("token")

		// Save a pre-generated CLI token (GitHub OAuth users or dashboard tokens).
		if token != "" {
			cfg := loadConfig()
			cfg.Token = token
			if apiFlag != "" {
				cfg.APIURL = apiFlag
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Println("Token saved.")
			return nil
		}

		email, _ := cmd.Flags().GetString("email")
		if email == "" {
			return fmt.Errorf("provide --token (from the web UI) or --email to sign in with a code")
		}

		// Persist API URL before making requests so do() picks it up.
		if apiFlag != "" {
			cfg := loadConfig()
			cfg.APIURL = apiFlag
			_ = saveConfig(cfg)
		}

		// Send OTP.
		res, err := do("POST", "/auth/otp/send", map[string]string{"email": email})
		if err != nil {
			return err
		}
		res.Body.Close()
		if res.StatusCode >= 400 {
			return fmt.Errorf("failed to send code (HTTP %d)", res.StatusCode)
		}

		fmt.Printf("Code sent to %s. Check your inbox.\n", email)
		fmt.Print("Enter code: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		code := strings.TrimSpace(scanner.Text())
		if len(code) == 0 {
			return fmt.Errorf("no code entered")
		}

		// Verify OTP.
		res, err = do("POST", "/auth/otp/verify", map[string]string{"email": email, "code": code})
		if err != nil {
			return err
		}
		var tok struct {
			Token   string `json:"token"`
			NewUser bool   `json:"new_user"`
		}
		if err := decodeOK(res, &tok); err != nil {
			return err
		}

		cfg := loadConfig()
		cfg.Token = tok.Token
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Logged in.")
		return nil
	},
}

var envsCmd = &cobra.Command{
	Use:   "envs",
	Short: "List environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := do("GET", "/environments", nil)
		if err != nil {
			return err
		}
		var envs []environment
		if err := decodeOK(res, &envs); err != nil {
			return err
		}
		if len(envs) == 0 {
			fmt.Println("No environments.")
			return nil
		}
		for _, e := range envs {
			fmt.Printf("%-36s  %-20s  %s\n", e.ID, e.Name, e.ClusterName)
		}
		return nil
	},
}

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "View the event timeline for an environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		envName, _ := cmd.Flags().GetString("env")
		if envName == "" {
			return fmt.Errorf("--env is required")
		}
		limit, _ := cmd.Flags().GetInt("limit")

		env, err := findEnv(envName)
		if err != nil {
			return err
		}

		res, err := do("GET", fmt.Sprintf("/events?environment_id=%s&limit=%d", env.ID, limit), nil)
		if err != nil {
			return err
		}
		var events []struct {
			ID        string                 `json:"id"`
			Type      string                 `json:"type"`
			Service   string                 `json:"service"`
			Timestamp string                 `json:"timestamp"`
			Metadata  map[string]interface{} `json:"metadata"`
		}
		if err := decodeOK(res, &events); err != nil {
			return err
		}
		if len(events) == 0 {
			fmt.Println("No events.")
			return nil
		}
		for _, ev := range events {
			ts, _ := time.Parse(time.RFC3339, ev.Timestamp)
			meta, _ := json.Marshal(ev.Metadata)
			fmt.Printf("[%s] %-8s  %-20s  %s  %s\n",
				ts.Format("15:04"), ev.Type, ev.Service, ts.Format("2006-01-02"), string(meta))
		}
		return nil
	},
}

var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Add a note event",
	RunE: func(cmd *cobra.Command, args []string) error {
		envName, _ := cmd.Flags().GetString("env")
		if envName == "" {
			return fmt.Errorf("--env is required")
		}
		if len(args) == 0 {
			return fmt.Errorf("note body is required")
		}
		body := args[0]

		env, err := findEnv(envName)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{
			"type":           "note",
			"service":        "manual",
			"environment_id": env.ID,
			"metadata":       map[string]string{"body": body},
		}
		res, err := do("POST", "/events", payload)
		if err != nil {
			return err
		}
		var ev struct {
			ID string `json:"id"`
		}
		if err := decodeOK(res, &ev); err != nil {
			return err
		}
		fmt.Printf("Note created: %s\n", ev.ID)
		return nil
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Send a deploy event",
	// The freeze exit is a signal, not a misuse of the command: the deploy was
	// recorded and already reported, so cobra must not add a usage block or
	// print the error a second time.
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		envName, _ := cmd.Flags().GetString("env")
		service, _ := cmd.Flags().GetString("service")
		version, _ := cmd.Flags().GetString("version")
		committedAt, _ := cmd.Flags().GetString("committed-at")

		if envName == "" || service == "" || version == "" {
			return fmt.Errorf("--env, --service, and --version are required")
		}

		metadata := map[string]string{"version": version, "author": os.Getenv("USER")}
		// committed_at feeds the DORA lead-time metric (commit -> deploy). It
		// must be RFC3339 so the server can diff it against the deploy time.
		if committedAt != "" {
			if _, err := time.Parse(time.RFC3339, committedAt); err != nil {
				return fmt.Errorf("--committed-at must be an RFC3339 timestamp (e.g. 2026-06-29T10:00:00Z)")
			}
			metadata["committed_at"] = committedAt
		}

		env, err := findEnv(envName)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{
			"type":           "deploy",
			"service":        service,
			"environment_id": env.ID,
			"metadata":       metadata,
		}
		res, err := do("POST", "/events", payload)
		if err != nil {
			return err
		}
		var ev struct {
			ID     string `json:"id"`
			Freeze *struct {
				Reason  string `json:"reason"`
				EndsAt  string `json:"ends_at"`
				OrgWide bool   `json:"org_wide"`
			} `json:"freeze"`
		}
		if err := decodeOK(res, &ev); err != nil {
			return err
		}
		fmt.Printf("Deploy event created: %s\n", ev.ID)

		// The deploy already happened — Kollaber records changes, it does not
		// gate them — so the event is always created and always reported. The
		// non-zero exit is for CI to act on, and --allow-frozen is how a release
		// that is meant to go out during a freeze says so deliberately.
		if ev.Freeze != nil {
			scope := "this environment is"
			if ev.Freeze.OrgWide {
				scope = "all environments are"
			}
			fmt.Fprintf(os.Stderr, "\nWARNING: change freeze in effect — %s frozen: %s\n", scope, ev.Freeze.Reason)
			if until, err := time.Parse(time.RFC3339, ev.Freeze.EndsAt); err == nil {
				fmt.Fprintf(os.Stderr, "         until %s\n", until.Local().Format("Mon 2 Jan 15:04"))
			}
			if allowFrozen, _ := cmd.Flags().GetBool("allow-frozen"); !allowFrozen {
				fmt.Fprintf(os.Stderr, "         pass --allow-frozen to exit zero anyway\n")
				return errFrozen
			}
		}
		return nil
	},
}

// errFrozen exits non-zero without cobra printing a usage block: the deploy was
// recorded successfully, so this is a signal to CI rather than a misuse of the
// command.
var errFrozen = &silentError{"deploy landed inside a change freeze"}

type silentError struct{ msg string }

func (e *silentError) Error() string { return e.msg }

// --- chat sessions ---

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// maxSessionMessages caps how much history we keep locally; the server also
// truncates, so there's no value in persisting more than this.
const maxSessionMessages = 20

func sessionPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kollaber", "session.json")
}

func loadSession() []chatMsg {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return nil
	}
	var s struct {
		Messages []chatMsg `json:"messages"`
	}
	_ = json.Unmarshal(data, &s)
	return s.Messages
}

func saveSession(messages []chatMsg) error {
	if len(messages) > maxSessionMessages {
		messages = messages[len(messages)-maxSessionMessages:]
	}
	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(struct {
		Messages []chatMsg `json:"messages"`
	}{messages}, "", "  ")
	return os.WriteFile(p, data, 0600)
}

// streamAsk sends the conversation to the agent and streams the reply: answer
// text to stdout, tool-lookup progress to stderr (unless quiet). It returns the
// full answer text so the caller can append it to the conversation history.
func streamAsk(envID string, messages []chatMsg, quiet bool) (string, error) {
	res, err := do("POST", "/ai/chat", map[string]any{
		"environment_id": envID,
		"messages":       messages,
	})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Gating failures (auth, quota, rate limit) come back as JSON, not a stream.
	if res.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&e)
		if e.Error != "" {
			return "", fmt.Errorf("%s", e.Error)
		}
		return "", fmt.Errorf("HTTP %d", res.StatusCode)
	}

	// Read the SSE stream: "token" chunks form the answer, "step" reports a
	// tool lookup, "error"/"done" end it.
	var answer strings.Builder
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[len("data:"):])
		if data == "" {
			continue
		}
		var ev struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Tool  string `json:"tool"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "token":
			fmt.Print(ev.Text)
			answer.WriteString(ev.Text)
		case "step":
			if !quiet {
				fmt.Fprintf(os.Stderr, "… %s\n", ev.Tool)
			}
		case "error":
			if answer.Len() > 0 {
				fmt.Println()
			}
			return "", fmt.Errorf("%s", ev.Error)
		case "done":
			fmt.Println()
			return answer.String(), nil
		}
	}
	if err := sc.Err(); err != nil {
		return answer.String(), fmt.Errorf("reading response: %w", err)
	}
	if answer.Len() > 0 {
		fmt.Println()
	}
	return answer.String(), nil
}

// askREPL runs an interactive multi-turn chat, holding history in memory until
// the user exits. Prompts and notices go to stderr so stdout stays pipeable.
func askREPL(envID string, quiet bool) error {
	fmt.Fprintln(os.Stderr, "Kollaber assistant — ask a question, or type 'exit' to quit.")
	var history []chatMsg
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Fprint(os.Stderr, "you › ")
		if !in.Scan() {
			fmt.Fprintln(os.Stderr)
			return in.Err() // nil on EOF (Ctrl-D), non-nil on read error
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}
		history = append(history, chatMsg{Role: "user", Content: line})
		answer, err := streamAsk(envID, history, quiet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			history = history[:len(history)-1] // drop the failed turn
			continue
		}
		history = append(history, chatMsg{Role: "assistant", Content: answer})
	}
}

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask the AI timeline assistant a question",
	Long: `Ask the AI timeline assistant a natural-language question about your events.

The answer streams to stdout; tool lookups are reported on stderr, so you can
pipe the answer cleanly.

  kollaber ask --env prod "what deployed in the last hour?"
  kollaber ask "summarize today's alerts" > summary.txt

Conversations persist across commands by default, so follow-ups work:

  kollaber ask "what was the last alert?"
  kollaber ask "yes, show its metadata"     # remembers the previous turn

Run with no question to open an interactive multi-turn session:

  kollaber ask --env prod

Use --new to start a fresh conversation and --no-save for a one-off question
that neither reads nor writes saved history.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		noSave, _ := cmd.Flags().GetBool("no-save")
		newConv, _ := cmd.Flags().GetBool("new")

		var envID string
		if envName, _ := cmd.Flags().GetString("env"); envName != "" {
			env, err := findEnv(envName)
			if err != nil {
				return err
			}
			envID = env.ID
		}

		// No question → interactive REPL (always a fresh in-memory conversation).
		if len(args) == 0 {
			return askREPL(envID, quiet)
		}

		question := strings.Join(args, " ")
		var history []chatMsg
		if !noSave && !newConv {
			history = loadSession()
		}
		history = append(history, chatMsg{Role: "user", Content: question})

		answer, err := streamAsk(envID, history, quiet)
		if err != nil {
			return err
		}
		if !noSave {
			history = append(history, chatMsg{Role: "assistant", Content: answer})
			_ = saveSession(history)
		}
		return nil
	},
}

// --- incident commands ---

type incidentResp struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Status     string `json:"status"`
	OpenedAt   string `json:"opened_at"`
	ResolvedAt string `json:"resolved_at"`
	EventCount int    `json:"event_count"`
}

var incidentCmd = &cobra.Command{
	Use:   "incident",
	Short: "Manage incidents",
}

var incidentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		path := "/incidents"
		if status != "" {
			path += "?status=" + status
		}
		res, err := do("GET", path, nil)
		if err != nil {
			return err
		}
		var incidents []incidentResp
		if err := decodeOK(res, &incidents); err != nil {
			return err
		}
		if len(incidents) == 0 {
			fmt.Println("No incidents.")
			return nil
		}
		for _, in := range incidents {
			opened, _ := time.Parse(time.RFC3339, in.OpenedAt)
			fmt.Printf("%-36s  %-4s  %-9s  %2d events  %s  %s\n",
				in.ID, strings.ToUpper(in.Severity), in.Status, in.EventCount,
				opened.Format("2006-01-02 15:04"), in.Title)
		}
		return nil
	},
}

var incidentOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a new incident",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		severity, _ := cmd.Flags().GetString("severity")
		eventIDs, _ := cmd.Flags().GetStringSlice("event")
		if title == "" {
			return fmt.Errorf("--title is required")
		}

		payload := map[string]interface{}{
			"title":     title,
			"severity":  severity,
			"event_ids": eventIDs,
		}
		res, err := do("POST", "/incidents", payload)
		if err != nil {
			return err
		}
		var in incidentResp
		if err := decodeOK(res, &in); err != nil {
			return err
		}
		fmt.Printf("Incident opened: %s\n", in.ID)
		return nil
	},
}

var incidentResolveCmd = &cobra.Command{
	Use:   "resolve <incident-id>",
	Short: "Change an incident's status (defaults to resolved)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		res, err := do("PATCH", "/incidents/"+args[0], map[string]string{"status": status})
		if err != nil {
			return err
		}
		var in incidentResp
		if err := decodeOK(res, &in); err != nil {
			return err
		}
		fmt.Printf("Incident %s → %s\n", in.ID, in.Status)
		return nil
	},
}

var incidentAttachCmd = &cobra.Command{
	Use:   "attach <incident-id>",
	Short: "Attach one or more events to an incident",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eventIDs, _ := cmd.Flags().GetStringSlice("event")
		if len(eventIDs) == 0 {
			return fmt.Errorf("at least one --event is required")
		}
		res, err := do("POST", "/incidents/"+args[0]+"/events", map[string]interface{}{"event_ids": eventIDs})
		if err != nil {
			return err
		}
		var out struct {
			Attached int `json:"attached"`
		}
		if err := decodeOK(res, &out); err != nil {
			return err
		}
		fmt.Printf("Attached %d event(s) to incident %s\n", out.Attached, args[0])
		return nil
	},
}

type doraMetricResp struct {
	Value   float64 `json:"value"`
	Display string  `json:"display"`
	Rating  string  `json:"rating"`
	Samples int64   `json:"samples"`
}

type doraResp struct {
	WindowDays      int            `json:"window_days"`
	DeployFrequency doraMetricResp `json:"deploy_frequency"`
	LeadTime        doraMetricResp `json:"lead_time"`
	ChangeFailRate  doraMetricResp `json:"change_failure_rate"`
	TimeToRestore   doraMetricResp `json:"time_to_restore"`
}

var doraCmd = &cobra.Command{
	Use:   "dora",
	Short: "Show DORA metrics (deploy frequency, lead time, change failure rate, time to restore)",
	RunE: func(cmd *cobra.Command, args []string) error {
		days, _ := cmd.Flags().GetInt("days")
		envName, _ := cmd.Flags().GetString("env")

		path := fmt.Sprintf("/metrics/dora?days=%d", days)
		if envName != "" {
			env, err := findEnv(envName)
			if err != nil {
				return err
			}
			path += "&environment_id=" + env.ID
		}

		res, err := do("GET", path, nil)
		if err != nil {
			return err
		}
		var d doraResp
		if err := decodeOK(res, &d); err != nil {
			return err
		}

		scope := "all environments"
		if envName != "" {
			scope = envName
		}
		fmt.Printf("DORA metrics — last %d days — %s\n\n", d.WindowDays, scope)
		row := func(label string, m doraMetricResp, note string) {
			fmt.Printf("  %-22s %-14s %-7s %s\n", label, m.Display, strings.ToUpper(m.Rating), note)
		}
		// Time to restore comes from incidents, which aren't tied to an
		// environment — so it's always org-wide. Flag that when a scope is set.
		mttrNote := ""
		if envName != "" {
			mttrNote = "(org-wide)"
		}
		row("Deployment frequency", d.DeployFrequency, "")
		row("Lead time for changes", d.LeadTime, "")
		row("Change failure rate", d.ChangeFailRate, "")
		row("Time to restore", d.TimeToRestore, mttrNote)
		return nil
	},
}

func init() {
	loginCmd.Flags().String("api", "", "API base URL (e.g. https://kollaber.io) — saved to config")
	loginCmd.Flags().String("token", "", "CLI token from the web UI (for GitHub OAuth users)")
	loginCmd.Flags().String("email", "", "Email address (sends a one-time code)")

	timelineCmd.Flags().String("env", "", "Environment name or ID")
	timelineCmd.Flags().Int("limit", 20, "Max events to show")

	noteCmd.Flags().String("env", "", "Environment name or ID")

	deployCmd.Flags().String("env", "", "Environment name or ID")
	deployCmd.Flags().String("service", "", "Service name")
	deployCmd.Flags().String("version", "", "Version string (e.g. v1.2.3)")
	deployCmd.Flags().String("committed-at", "", "Commit time (RFC3339) — powers the DORA lead-time metric")
	deployCmd.Flags().Bool("allow-frozen", false, "Exit zero even if the deploy lands inside a change freeze")

	askCmd.Flags().String("env", "", "Scope the question to an environment name or ID")
	askCmd.Flags().Bool("quiet", false, "Suppress tool-lookup progress on stderr")
	askCmd.Flags().Bool("new", false, "Start a fresh conversation, ignoring saved history")
	askCmd.Flags().Bool("no-save", false, "Don't read or write saved conversation history")

	incidentListCmd.Flags().String("status", "", "Filter by status: open, mitigated, resolved")
	incidentOpenCmd.Flags().String("title", "", "Incident title")
	incidentOpenCmd.Flags().String("severity", "sev3", "Severity: sev1, sev2, sev3, sev4")
	incidentOpenCmd.Flags().StringSlice("event", nil, "Event ID to attach (repeatable)")
	incidentResolveCmd.Flags().String("status", "resolved", "New status: open, mitigated, resolved")
	incidentAttachCmd.Flags().StringSlice("event", nil, "Event ID to attach (repeatable)")
	incidentCmd.AddCommand(incidentListCmd, incidentOpenCmd, incidentResolveCmd, incidentAttachCmd)

	doraCmd.Flags().Int("days", 30, "Window size in days")
	doraCmd.Flags().String("env", "", "Scope to an environment name or ID (omit for all)")

	rootCmd.AddCommand(loginCmd, envsCmd, timelineCmd, noteCmd, deployCmd, askCmd, incidentCmd, doraCmd, mcpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Exit 2 distinguishes "landed in a freeze" from any other failure, so
		// a pipeline can fail the build on a freeze while still telling it
		// apart from a network error or a bad flag.
		var frozen *silentError
		if errors.As(err, &frozen) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
