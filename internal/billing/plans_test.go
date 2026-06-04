package billing

import "testing"

func TestEntitlementsFor(t *testing.T) {
	tests := []struct {
		plan        string
		wantAgent   bool
		wantQuota   int
		wantHistory int
	}{
		{PlanFree, false, 0, 30},
		{PlanTeam, true, 200, -1},
		{PlanPro, true, 1000, -1},
		{PlanEnterprise, true, -1, -1},
		{"nonsense-plan", false, 0, 30}, // unknown plans fall back to Free
	}
	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			e := EntitlementsFor(tt.plan)
			if e.AIAgent != tt.wantAgent {
				t.Errorf("AIAgent = %v, want %v", e.AIAgent, tt.wantAgent)
			}
			if e.AIAgentMonthlyQuota != tt.wantQuota {
				t.Errorf("AIAgentMonthlyQuota = %d, want %d", e.AIAgentMonthlyQuota, tt.wantQuota)
			}
			if e.HistoryDays != tt.wantHistory {
				t.Errorf("HistoryDays = %d, want %d", e.HistoryDays, tt.wantHistory)
			}
		})
	}
}

func TestEntitlementsFor_SelfHostedUnlocksEverything(t *testing.T) {
	t.Setenv("SELF_HOSTED", "true")
	// Even a Free plan should get unlimited agent access when self-hosted.
	e := EntitlementsFor(PlanFree)
	if !e.AIAgent {
		t.Error("self-hosted Free should have AIAgent enabled")
	}
	if e.AIAgentMonthlyQuota != -1 {
		t.Errorf("self-hosted quota = %d, want -1 (unlimited)", e.AIAgentMonthlyQuota)
	}
}
