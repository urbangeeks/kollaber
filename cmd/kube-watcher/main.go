// kube-watcher watches a Kubernetes cluster and fires Kollaber events for
// Deployment/StatefulSet/DaemonSet rollouts (and teardowns) and
// CrashLoopBackOff pods.
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
	kubeconfig = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "path to kubeconfig file")
	envName    = flag.String("env", "", "Kollaber environment name (required)")
	apiURL     = flag.String("api", os.Getenv("KOLLABER_API"), "Kollaber API base URL")
	token      = flag.String("token", os.Getenv("KOLLABER_TOKEN"), "Kollaber CLI token")
	namespace  = flag.String("namespace", "", "Kubernetes namespace to watch (empty = all namespaces)")
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
	replicas   int32
}

// watchWorkload fires a deploy event when a workload completes a rollout and,
// when --report-deletes is set, a teardown event when one is removed.
func watchWorkload(ctx context.Context, k *kollaberClient, a workloadAdapter) {
	// Track the last generation we fired per object (namespace/name → generation).
	// Generation increments on every spec change, so each rollout fires exactly once.
	fired := map[string]int64{}

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
				// Forget the generation so a later re-create fires a fresh deploy event.
				delete(fired, key)
				log.Printf("teardown: %s %s namespace=%s", a.kind, m.name, m.namespace)
				if err := k.sendEvent("teardown", m.name, map[string]any{
					"namespace": m.namespace,
					"kind":      a.kind,
					"version":   m.image,
				}); err != nil {
					log.Printf("error sending teardown event for %s: %v", m.name, err)
				}

			case watch.Modified:
				if !m.ready {
					continue
				}
				if fired[key] == m.generation {
					continue
				}
				fired[key] = m.generation
				log.Printf("deploy: %s %s image=%s generation=%d", a.kind, m.name, m.image, m.generation)
				if err := k.sendEvent("deploy", m.name, map[string]any{
					"version":   m.image,
					"namespace": m.namespace,
					"kind":      a.kind,
					"replicas":  m.replicas,
				}); err != nil {
					log.Printf("error sending deploy event for %s: %v", m.name, err)
				}
			}
		}
	}
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
			}, true
		},
	}
}

// watchPods fires an alert event when a pod enters CrashLoopBackOff.
func watchPods(ctx context.Context, client kubernetes.Interface, k *kollaberClient) {
	// Track pods we've already alerted on so we don't spam.
	alerted := map[string]bool{}

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
			if event.Type != watch.Modified {
				continue
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			key := pod.Namespace + "/" + pod.Name
			if alerted[key] {
				continue
			}
			if container, crash := crashLoopContainer(pod); crash {
				alerted[key] = true
				service := podServiceName(pod)
				log.Printf("alert: CrashLoopBackOff pod=%s container=%s service=%s", pod.Name, container, service)
				if err := k.sendEvent("alert", service, map[string]any{
					"reason":    "CrashLoopBackOff",
					"pod":       pod.Name,
					"container": container,
					"namespace": pod.Namespace,
				}); err != nil {
					log.Printf("error sending alert event for pod %s: %v", pod.Name, err)
				}
			} else {
				// Clear alert once pod recovers.
				delete(alerted, key)
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

func crashLoopContainer(pod *corev1.Pod) (string, bool) {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			return cs.Name, true
		}
	}
	return "", false
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
		var e struct{ Error string `json:"error"` }
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
