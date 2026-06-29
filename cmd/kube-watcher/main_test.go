package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestImageTag(t *testing.T) {
	cases := map[string]string{
		"nginx:1.25":                      "1.25",
		"nginx":                           "latest",
		"registry.io/team/app:v1.2.3":     "v1.2.3",
		"localhost:5000/app:dev":          "dev", // registry port must not be read as the tag
		"localhost:5000/app":              "latest",
		"app@sha256:abcdef0123456789aaaa": "sha256:abcdef012345", // digest truncated to "sha256:" + 12 hex
	}
	for image, want := range cases {
		if got := imageTag(image); got != want {
			t.Errorf("imageTag(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestClassifyChange(t *testing.T) {
	// First sighting of a workload is always a deploy.
	st := &workloadState{seenImages: map[string]bool{}}
	m := workloadMeta{namespace: "default", name: "api", image: "api:v1", replicas: 3, desired: 3}
	if typ, _ := classifyChange("deployment", m, st); typ != "deploy" {
		t.Fatalf("first sighting = %q, want deploy", typ)
	}
	// Simulate the watch loop recording state after firing.
	st.firedGen, st.image, st.desired = 1, "api:v1", 3
	st.seenImages["api:v1"] = true

	// New image we've never seen → deploy.
	m2 := m
	m2.image = "api:v2"
	if typ, meta := classifyChange("deployment", m2, st); typ != "deploy" {
		t.Errorf("forward change = %q, want deploy", typ)
	} else if meta["previous_image"] != "api:v1" {
		t.Errorf("previous_image = %v, want api:v1", meta["previous_image"])
	}
	st.firedGen, st.image = 2, "api:v2"
	st.seenImages["api:v2"] = true

	// Image reverting to one we've seen → rollback.
	m3 := m
	m3.image = "api:v1"
	if typ, _ := classifyChange("deployment", m3, st); typ != "rollback" {
		t.Errorf("revert to seen image = %q, want rollback", typ)
	}
	st.firedGen, st.image = 3, "api:v1"

	// Same image, replica count up → scale up.
	m4 := m
	m4.desired, m4.replicas = 5, 5
	typ, meta := classifyChange("deployment", m4, st)
	if typ != "scale" {
		t.Fatalf("replica change = %q, want scale", typ)
	}
	if meta["direction"] != "up" {
		t.Errorf("direction = %v, want up", meta["direction"])
	}
	if meta["previous_replicas"] != int32(3) {
		t.Errorf("previous_replicas = %v, want 3", meta["previous_replicas"])
	}
}

func TestPodProblem(t *testing.T) {
	waiting := func(reason string) *corev1.Pod {
		return &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}},
				},
			},
		}
	}

	for _, reason := range []string{"CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull"} {
		c, got, ok := podProblem(waiting(reason))
		if !ok || got != reason || c != "app" {
			t.Errorf("podProblem(%s) = (%q,%q,%v), want (app,%s,true)", reason, c, got, ok, reason)
		}
	}

	// OOMKilled on the previous termination state.
	oom := &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
		{Name: "app", LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"}}},
	}}}
	if c, got, ok := podProblem(oom); !ok || got != "OOMKilled" || c != "app" {
		t.Errorf("podProblem(OOM) = (%q,%q,%v), want (app,OOMKilled,true)", c, got, ok)
	}

	// Unschedulable pending pod → pod-level FailedScheduling (no container).
	sched := &corev1.Pod{Status: corev1.PodStatus{
		Phase:      corev1.PodPending,
		Conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable"}},
	}}
	if c, got, ok := podProblem(sched); !ok || got != "FailedScheduling" || c != "" {
		t.Errorf("podProblem(unschedulable) = (%q,%q,%v), want (\"\",FailedScheduling,true)", c, got, ok)
	}

	// Healthy running pod → no problem.
	healthy := &corev1.Pod{Status: corev1.PodStatus{
		Phase:             corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "app", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}}}},
	}}
	if _, _, ok := podProblem(healthy); ok {
		t.Error("podProblem(healthy) = true, want false")
	}
}
