package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		envName, _ := cmd.Flags().GetString("env")
		service, _ := cmd.Flags().GetString("service")
		version, _ := cmd.Flags().GetString("version")

		if envName == "" || service == "" || version == "" {
			return fmt.Errorf("--env, --service, and --version are required")
		}

		env, err := findEnv(envName)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{
			"type":           "deploy",
			"service":        service,
			"environment_id": env.ID,
			"metadata":       map[string]string{"version": version, "author": os.Getenv("USER")},
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
		fmt.Printf("Deploy event created: %s\n", ev.ID)
		return nil
	},
}

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
			return nil // EOF (Ctrl-D)
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

	askCmd.Flags().String("env", "", "Scope the question to an environment name or ID")
	askCmd.Flags().Bool("quiet", false, "Suppress tool-lookup progress on stderr")
	askCmd.Flags().Bool("new", false, "Start a fresh conversation, ignoring saved history")
	askCmd.Flags().Bool("no-save", false, "Don't read or write saved conversation history")

	rootCmd.AddCommand(loginCmd, envsCmd, timelineCmd, noteCmd, deployCmd, askCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
