package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

const defaultAPI = "http://localhost:8080"

// --- config ---

type config struct {
	Token   string `json:"token"`
	APIURL  string `json:"api_url"`
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
		var e struct{ Error string `json:"error"` }
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

var rootCmd = &cobra.Command{
	Use:   "kollaber",
	Short: "Kollaber CLI — infrastructure event timeline",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and save a token",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		if token != "" {
			cfg := loadConfig()
			cfg.Token = token
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Println("Token saved.")
			return nil
		}

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		if email == "" || password == "" {
			return fmt.Errorf("provide --token (from the web UI) or both --email and --password")
		}

		res, err := do("POST", "/auth/login", map[string]string{"email": email, "password": password})
		if err != nil {
			return err
		}
		var tok struct{ Token string `json:"token"` }
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
		var ev struct{ ID string `json:"id"` }
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
		var ev struct{ ID string `json:"id"` }
		if err := decodeOK(res, &ev); err != nil {
			return err
		}
		fmt.Printf("Deploy event created: %s\n", ev.ID)
		return nil
	},
}

func init() {
	loginCmd.Flags().String("token", "", "CLI token from the web UI (for GitHub OAuth users)")
	loginCmd.Flags().String("email", "", "Email address")
	loginCmd.Flags().String("password", "", "Password")

	timelineCmd.Flags().String("env", "", "Environment name or ID")
	timelineCmd.Flags().Int("limit", 20, "Max events to show")

	noteCmd.Flags().String("env", "", "Environment name or ID")

	deployCmd.Flags().String("env", "", "Environment name or ID")
	deployCmd.Flags().String("service", "", "Service name")
	deployCmd.Flags().String("version", "", "Version string (e.g. v1.2.3)")

	rootCmd.AddCommand(loginCmd, envsCmd, timelineCmd, noteCmd, deployCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
