package api

import (
	"encoding/json"
	"testing"
)

// A verbatim Atlantis apply webhook, used to pin the json tags. Atlantis
// marshals its ApplyResult struct without tags, so the wire format is
// capitalised Go field names — the one shape a lowercase tag would silently
// fail to read.
const sampleAtlantisDelivery = `{
  "Workspace": "default",
  "Repo": {
    "FullName": "octocat/Hello-World",
    "Owner": "octocat",
    "Name": "Hello-World",
    "CloneURL": "https://:@github.com/octocat/Hello-World.git",
    "SanitizedCloneURL": "https://:<redacted>@github.com/octocat/Hello-World.git",
    "VCSHost": {
      "Hostname": "github.com",
      "Type": 0
    }
  },
  "Pull": {
    "Num": 2137,
    "HeadCommit": "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d",
    "URL": "https://github.com/octocat/Hello-World/pull/2137",
    "HeadBranch": "feature/some-branch",
    "BaseBranch": "main",
    "Author": "octocat",
    "Body": "This is the pull request description.",
    "State": 0,
    "BaseRepo": {
      "FullName": "octocat/Hello-World",
      "Owner": "octocat",
      "Name": "Hello-World",
      "CloneURL": "https://:@github.com/octocat/Hello-World.git",
      "SanitizedCloneURL": "https://:<redacted>@github.com/octocat/Hello-World.git",
      "VCSHost": {
        "Hostname": "github.com",
        "Type": 0
      }
    }
  },
  "User": {
    "Username": "octocat",
    "Teams": null
  },
  "Success": true,
  "Directory": "terraform/example",
  "ProjectName": "example-project"
}`

func TestParseAtlantisPayload(t *testing.T) {
	var got atlantisPayload
	if err := json.Unmarshal([]byte(sampleAtlantisDelivery), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Workspace != "default" {
		t.Errorf("Workspace = %q", got.Workspace)
	}
	if !got.Success {
		t.Error("Success did not parse")
	}
	if got.Directory != "terraform/example" {
		t.Errorf("Directory = %q", got.Directory)
	}
	if got.ProjectName != "example-project" {
		t.Errorf("ProjectName = %q", got.ProjectName)
	}
	if got.Repo.FullName != "octocat/Hello-World" {
		t.Errorf("Repo.FullName = %q", got.Repo.FullName)
	}
	if got.Pull.Num != 2137 {
		t.Errorf("Pull.Num = %d", got.Pull.Num)
	}
	if got.Pull.HeadCommit != "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d" {
		t.Errorf("Pull.HeadCommit = %q", got.Pull.HeadCommit)
	}
	if got.Pull.HeadBranch != "feature/some-branch" {
		t.Errorf("Pull.HeadBranch = %q", got.Pull.HeadBranch)
	}
	if got.User.Username != "octocat" {
		t.Errorf("User.Username = %q", got.User.Username)
	}
}

func TestAtlantisService(t *testing.T) {
	full := atlantisPayload{
		ProjectName: "example-project",
		Directory:   "terraform/example",
		Repo:        atlantisRepo{Name: "Hello-World", FullName: "octocat/Hello-World"},
	}

	tests := []struct {
		name    string
		payload atlantisPayload
		want    string
	}{
		{"prefers the project name", full, "example-project"},
		{
			// Set only when the repo declares projects in atlantis.yaml.
			name:    "falls back to the directory",
			payload: atlantisPayload{Directory: "terraform/example", Repo: full.Repo},
			want:    "terraform/example",
		},
		{
			// A root-level module reports "." as its directory, which names
			// nothing a person would recognise on a timeline.
			name:    "root directory is not a service name",
			payload: atlantisPayload{Directory: ".", Repo: full.Repo},
			want:    "Hello-World",
		},
		{
			name:    "falls back to the repo name",
			payload: atlantisPayload{Repo: full.Repo},
			want:    "Hello-World",
		},
		{
			name:    "falls back to the full name",
			payload: atlantisPayload{Repo: atlantisRepo{FullName: "octocat/Hello-World"}},
			want:    "octocat/Hello-World",
		},
		{"blank fields are skipped", atlantisPayload{ProjectName: "   ", Directory: "infra"}, "infra"},
		{"nothing usable", atlantisPayload{}, "terraform"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := atlantisService(tt.payload); got != tt.want {
				t.Errorf("atlantisService() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Atlantis reports a failed apply with Success:false rather than by omitting
// the field, so the zero value has to mean failure.
func TestAtlantisFailedApplyIsNotSuccess(t *testing.T) {
	var got atlantisPayload
	if err := json.Unmarshal([]byte(`{"Workspace":"default","Success":false}`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Success {
		t.Error("a failed apply parsed as successful")
	}
}
