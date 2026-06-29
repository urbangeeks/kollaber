// kube-watcher watches a Kubernetes cluster and fires Kollaber events for
// Deployment/StatefulSet/DaemonSet workloads — deploys, rollbacks (image
// reverting to a previously seen tag), scales (replica changes), and teardowns —
// plus pod failures: CrashLoopBackOff, image pull errors, OOM kills, and
// unschedulable pods. Deploy events carry replica counts, image tag, and rollout
// duration.
//
// Run one instance per cluster:
//
//	kube-watcher --kubeconfig ~/.kube/config --env prod --api https://kollaber.io --token <cli-token>
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig    = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "path to kubeconfig file")
	envName       = flag.String("env", "", "Kollaber environment name (required)")
	apiURL        = flag.String("api", os.Getenv("KOLLABER_API"), "Kollaber API base URL")
	token         = flag.String("token", os.Getenv("KOLLABER_TOKEN"), "Kollaber CLI token")
	namespace     = flag.String("namespace", "", "Kubernetes namespace to watch (empty = all namespaces)")
	reportDeletes = flag.Bool("report-deletes", false, "also fire a teardown event when a Deployment is removed (off by default — helm uninstall can produce a burst of events)")
)

func main() {
	flag.Parse()

	if *envName == "" {
		log.Fatal("--env is required")
	}
	if *apiURL == "" {
		log.Fatal("--api is required (or set KOLLABER_API)")
	}
	if *token == "" {
		log.Fatal("--token is required (or set KOLLABER_TOKEN)")
	}
	// Try in-cluster config first (when running as a Pod), then fall back to kubeconfig.
	cfg, err := rest.InClusterConfig()
	if err != nil {
		if *kubeconfig == "" {
			home, _ := os.UserHomeDir()
			*kubeconfig = home + "/.kube/config"
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatalf("kubeconfig: %v", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}

	kollaber := &kollaberClient{apiURL: *apiURL, token: *token}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("watching cluster for env=%q namespace=%q api=%q", *envName, *namespace, *apiURL)

	go watchWorkload(ctx, kollaber, deploymentAdapter(client))
	go watchWorkload(ctx, kollaber, statefulSetAdapter(client))
	go watchWorkload(ctx, kollaber, daemonSetAdapter(client))
	go watchPods(ctx, client, kollaber)

	<-ctx.Done()
	log.Println("shutting down")
}

// watchDeployments fires a deploy event whenever a Deployment completes a rollout.
// workloadAdapter lets one watch loop handle any workload controller —
// Deployment, StatefulSet, DaemonSet — since they share rollout/teardown
// semantics. kind appears in logs and event metadata.
type workloadAdapter struct {
	kind  string
	list  func(ctx context.Context) (resourceVersion string, err error)
	watch func(ctx context.Context, resourceVersion string) (watch.Interface, error)
	info  func(obj runtime.Object) (workloadMeta, bool)
}

type workloadMeta struct {
	namespace  string
	name       string
	generation int64
	image      string
	ready      bool
	replicas   int32 // ready replicas
	desired    int32 // spec replicas (DaemonSet: desired scheduled)
}

// workloadState is the last-fired snapshot of one workload, used to classify
// the next change: an image change is a deploy (or rollback if we've seen that
// image before), a replica change with the same image is a scale event, and a
// new generation observation starts the rollout-duration clock.
type workloadState struct {
	firedGen     int64
	image        string
	desired      int32
	seenImages   map[string]bool
	rolloutGen   int64
	rolloutStart time.Time
}

// watchWorkload fires a deploy event when a workload completes a rollout and,
// when --report-deletes is set, a teardown event when one is removed.
func watchWorkload(ctx context.Context, k *kollaberClient, a workloadAdapter) {
	// Per-workload state (namespace/name → snapshot) so we can tell a deploy from
	// a rollback from a scale, and time the rollout. Persists across watch
	// reconnects below; lost on process restart, which is fine — the initial LIST
	// resync means existing workloads aren't replayed as fresh events.
	state := map[string]*workloadState{}

	for {
		if ctx.Err() != nil {
			return
		}
		// List first to capture the current ResourceVersion so the watch only
		// sees changes after the watcher starts, not existing state.
		rv, err := a.list(ctx)
		if err != nil {
			log.Printf("%s list error: %v — retrying in 10s", a.kind, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}
		watcher, err := a.watch(ctx, rv)
		if err != nil {
			log.Printf("%s watch error: %v — retrying in 10s", a.kind, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}

		for event := range watcher.ResultChan() {
			m, ok := a.info(event.Object)
			if !ok {
				continue
			}
			key := m.namespace + "/" + m.name

			switch event.Type {
			case watch.Deleted:
				if !*reportDeletes {
					continue
				}
				// Forget state so a later re-create fires a fresh deploy event.
				delete(state, key)
				log.Printf("teardown: %s %s namespace=%s", a.kind, m.name, m.namespace)
				if err := k.sendEvent("teardown", m.name, map[string]any{
					"namespace": m.namespace,
					"kind":      a.kind,
					"version":   m.image,
				}); err != nil {
					log.Printf("error sending teardown event for %s: %v", m.name, err)
				}

			case watch.Added, watch.Modified:
				st := state[key]
				if st == nil {
					st = &workloadState{seenImages: map[string]bool{}}
					state[key] = st
				}
				// Start (or restart) the rollout clock the moment a new spec
				// generation appears, before it has become ready.
				if m.generation != st.rolloutGen {
					st.rolloutGen = m.generation
					st.rolloutStart = time.Now()
				}
				if !m.ready {
					continue
				}
				// Only act once per generation; repeated status updates for an
				// already-ready generation carry the same generation number.
				if st.firedGen == m.generation {
					continue
				}

				eventType, meta := classifyChange(a.kind, m, st)
				st.firedGen = m.generation
				st.image = m.image
				st.desired = m.desired
				st.seenImages[m.image] = true

				log.Printf("%s: %s %s image=%s generation=%d", eventType, a.kind, m.name, m.image, m.generation)
				if err := k.sendEvent(eventType, m.name, meta); err != nil {
					log.Printf("error sending %s event for %s: %v", eventType, m.name, err)
				}
			}
		}
	}
}

// classifyChange decides whether a ready workload change is a deploy, a
// rollback (image reverting to one we've already seen this session), or a scale
// (replica count changed with the same image), and builds the event metadata.
func classifyChange(kind string, m workloadMeta, st *workloadState) (string, map[string]any) {
	meta := map[string]any{
		"namespace":        m.namespace,
		"kind":             kind,
		"version":          m.image,
		"image_tag":        imageTag(m.image),
		"replicas":         m.replicas,
		"replicas_desired": m.desired,
	}
	if !st.rolloutStart.IsZero() {
		meta["rollout_seconds"] = int(time.Since(st.rolloutStart).Seconds())
	}

	switch {
	case m.image != st.image && st.image != "":
		// Image changed on a workload we've seen before.
		meta["previous_image"] = st.image
		if st.seenImages[m.image] {
			return "rollback", meta
		}
		return "deploy", meta
	case m.image == st.image && st.firedGen != 0 && m.desired != st.desired:
		// Same image, replica count moved → a scale (manual or HPA-driven).
		meta["previous_replicas"] = st.desired
		if m.desired > st.desired {
			meta["direction"] = "up"
		} else {
			meta["direction"] = "down"
		}
		return "scale", meta
	default:
		// First sighting, or a spec bump that's neither an image nor replica
		// change — treat as a deploy.
		return "deploy", meta
	}
}

// imageTag extracts the tag (or a short digest) from a container image ref,
// handling registry ports (host:5000/img:tag) and digests (img@sha256:…).
func imageTag(image string) string {
	if at := strings.LastIndex(image, "@"); at != -1 {
		digest := image[at+1:]
		if len(digest) > 19 { // "sha256:" + 12 hex chars
			return digest[:19]
		}
		return digest
	}
	slash := strings.LastIndex(image, "/")
	if colon := strings.LastIndex(image, ":"); colon > slash {
		return image[colon+1:]
	}
	return "latest"
}

// int32OrOne dereferences an optional replica count, treating nil as 1 (the
// Kubernetes default when spec.replicas is unset).
func int32OrOne(p *int32) int32 {
	if p == nil {
		return 1
	}
	return *p
}

func deploymentAdapter(client kubernetes.Interface) workloadAdapter {
	c := client.AppsV1().Deployments(*namespace)
	return workloadAdapter{
		kind: "deployment",
		list: func(ctx context.Context) (string, error) {
			l, err := c.List(ctx, metav1.ListOptions{})
			if err != nil {
				return "", err
			}
			return l.ResourceVersion, nil
		},
		watch: func(ctx context.Context, rv string) (watch.Interface, error) {
			return c.Watch(ctx, metav1.ListOptions{ResourceVersion: rv})
		},
		info: func(obj runtime.Object) (workloadMeta, bool) {
			d, ok := obj.(*appsv1.Deployment)
			if !ok {
				return workloadMeta{}, false
			}
			return workloadMeta{
				namespace:  d.Namespace,
				name:       d.Name,
				generation: d.Generation,
				image:      containerImage(d.Spec.Template.Spec.Containers),
				ready: d.Status.ReadyReplicas > 0 &&
					d.Status.ReadyReplicas == d.Status.Replicas &&
					d.Status.UnavailableReplicas == 0,
				replicas: d.Status.ReadyReplicas,
				desired:  int32OrOne(d.Spec.Replicas),
			}, true
		},
	}
}

func statefulSetAdapter(client kubernetes.Interface) workloadAdapter {
	c := client.AppsV1().StatefulSets(*namespace)
	return workloadAdapter{
		kind: "statefulset",
		list: func(ctx context.Context) (string, error) {
			l, err := c.List(ctx, metav1.ListOptions{})
			if err != nil {
				return "", err
			}
			return l.ResourceVersion, nil
		},
		watch: func(ctx context.Context, rv string) (watch.Interface, error) {
			return c.Watch(ctx, metav1.ListOptions{ResourceVersion: rv})
		},
		info: func(obj runtime.Object) (workloadMeta, bool) {
			s, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				return workloadMeta{}, false
			}
			return workloadMeta{
				namespace:  s.Namespace,
				name:       s.Name,
				generation: s.Generation,
				image:      containerImage(s.Spec.Template.Spec.Containers),
				ready: s.Status.ReadyReplicas > 0 &&
					s.Status.ReadyReplicas == s.Status.Replicas &&
					s.Status.CurrentRevision == s.Status.UpdateRevision,
				replicas: s.Status.ReadyReplicas,
				desired:  int32OrOne(s.Spec.Replicas),
			}, true
		},
	}
}

func daemonSetAdapter(client kubernetes.Interface) workloadAdapter {
	c := client.AppsV1().DaemonSets(*namespace)
	return workloadAdapter{
		kind: "daemonset",
		list: func(ctx context.Context) (string, error) {
			l, err := c.List(ctx, metav1.ListOptions{})
			if err != nil {
				return "", err
			}
			return l.ResourceVersion, nil
		},
		watch: func(ctx context.Context, rv string) (watch.Interface, error) {
			return c.Watch(ctx, metav1.ListOptions{ResourceVersion: rv})
		},
		info: func(obj runtime.Object) (workloadMeta, bool) {
			d, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				return workloadMeta{}, false
			}
			return workloadMeta{
				namespace:  d.Namespace,
				name:       d.Name,
				generation: d.Generation,
				image:      containerImage(d.Spec.Template.Spec.Containers),
				ready: d.Status.DesiredNumberScheduled > 0 &&
					d.Status.NumberReady == d.Status.DesiredNumberScheduled &&
					d.Status.UpdatedNumberScheduled == d.Status.DesiredNumberScheduled,
				replicas: d.Status.NumberReady,
				desired:  d.Status.DesiredNumberScheduled,
			}, true
		},
	}
}

// watchPods fires an alert event when a pod hits a failure condition —
// CrashLoopBackOff, an image pull failure, an OOM kill, or being unschedulable.
func watchPods(ctx context.Context, client kubernetes.Interface, k *kollaberClient) {
	// Track the reason we last alerted per pod so we don't spam, but still
	// re-alert if the failure reason changes (e.g. ImagePull → CrashLoop).
	alerted := map[string]string{}

	for {
		if ctx.Err() != nil {
			return
		}
		list, err := client.CoreV1().Pods(*namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("pod list error: %v — retrying in 10s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}
		watcher, err := client.CoreV1().Pods(*namespace).Watch(ctx, metav1.ListOptions{
			ResourceVersion: list.ResourceVersion,
		})
		if err != nil {
			log.Printf("pod watch error: %v — retrying in 10s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}

		for event := range watcher.ResultChan() {
			// Added covers pods that arrive already unschedulable; Modified covers
			// containers that fail after starting.
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			key := pod.Namespace + "/" + pod.Name
			container, reason, problem := podProblem(pod)
			if !problem {
				// Recovered (or never failing) — clear so a future failure alerts.
				delete(alerted, key)
				continue
			}
			if alerted[key] == reason {
				continue // already alerted for this same reason
			}
			alerted[key] = reason
			service := podServiceName(pod)
			log.Printf("alert: %s pod=%s container=%s service=%s", reason, pod.Name, container, service)
			meta := map[string]any{
				"reason":    reason,
				"pod":       pod.Name,
				"namespace": pod.Namespace,
			}
			if container != "" {
				meta["container"] = container
			}
			if err := k.sendEvent("alert", service, meta); err != nil {
				log.Printf("error sending alert event for pod %s: %v", pod.Name, err)
			}
		}
	}
}

func containerImage(containers []corev1.Container) string {
	if len(containers) == 0 {
		return "unknown"
	}
	return containers[0].Image
}

// podServiceName returns the best service name for a pod, trying common label
// conventions before falling back to the pod name.
func podServiceName(pod *corev1.Pod) string {
	for _, key := range []string{"app", "app.kubernetes.io/name", "app.kubernetes.io/component"} {
		if v := pod.Labels[key]; v != "" {
			return v
		}
	}
	return pod.Name
}

// podProblem reports the first failure condition found on a pod and the
// container it belongs to ("" for pod-level problems like scheduling). It covers
// the common "why is this broken" reasons: crash loops, image pull failures,
// OOM kills, and unschedulable pods.
func podProblem(pod *corev1.Pod) (container, reason string, ok bool) {
	for _, cs := range pod.Status.ContainerStatuses {
		if w := cs.State.Waiting; w != nil {
			switch w.Reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
				"CreateContainerConfigError", "CreateContainerError":
				return cs.Name, w.Reason, true
			}
		}
		// OOM shows up on the current or previous termination state.
		if t := cs.State.Terminated; t != nil && t.Reason == "OOMKilled" {
			return cs.Name, "OOMKilled", true
		}
		if t := cs.LastTerminationState.Terminated; t != nil && t.Reason == "OOMKilled" {
			return cs.Name, "OOMKilled", true
		}
	}
	// Pod-level: pending and rejected by the scheduler.
	if pod.Status.Phase == corev1.PodPending {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled &&
				cond.Status == corev1.ConditionFalse &&
				cond.Reason == "Unschedulable" {
				return "", "FailedScheduling", true
			}
		}
	}
	return "", "", false
}

type kollaberClient struct {
	apiURL string
	token  string
	envID  string
}

type eventPayload struct {
	Type          string         `json:"type"`
	Service       string         `json:"service"`
	EnvironmentID string         `json:"environment_id"`
	Metadata      map[string]any `json:"metadata"`
	Status        string         `json:"status"`
}

func (k *kollaberClient) sendEvent(eventType, service string, metadata map[string]any) error {
	if k.envID == "" {
		id, err := k.resolveEnvID()
		if err != nil {
			return fmt.Errorf("resolve env %q: %w", *envName, err)
		}
		k.envID = id
	}

	status := "success"
	if eventType == "alert" {
		status = "failure"
	}

	payload := eventPayload{
		Type:          eventType,
		Service:       service,
		EnvironmentID: k.envID,
		Metadata:      metadata,
		Status:        status,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", k.apiURL+"/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("kollaber api %d: %s", resp.StatusCode, e.Error)
	}
	return nil
}

func (k *kollaberClient) resolveEnvID() (string, error) {
	req, err := http.NewRequest("GET", k.apiURL+"/environments", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var envs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envs); err != nil {
		return "", err
	}
	for _, e := range envs {
		if e.Name == *envName {
			return e.ID, nil
		}
	}
	return "", fmt.Errorf("environment %q not found", *envName)
}
