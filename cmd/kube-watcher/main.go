// kube-watcher watches a Kubernetes cluster and fires Kollaber events for
// Deployment rollouts and CrashLoopBackOff pods.
//
// Run one instance per cluster:
//
//	kube-watcher --kubeconfig ~/.kube/config --env prod --api https://api.kollaber.io --token <cli-token>
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

	go watchDeployments(ctx, client, kollaber)
	go watchPods(ctx, client, kollaber)

	<-ctx.Done()
	log.Println("shutting down")
}

// watchDeployments fires a deploy event whenever a Deployment completes a rollout.
func watchDeployments(ctx context.Context, client kubernetes.Interface, k *kollaberClient) {
	for {
		if ctx.Err() != nil {
			return
		}
		// List first to get the current ResourceVersion so the watch only
		// sees changes that happen after the watcher starts, not existing state.
		list, err := client.AppsV1().Deployments(*namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("deployment list error: %v — retrying in 10s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}
		watcher, err := client.AppsV1().Deployments(*namespace).Watch(ctx, metav1.ListOptions{
			ResourceVersion: list.ResourceVersion,
		})
		if err != nil {
			log.Printf("deployment watch error: %v — retrying in 10s", err)
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
			dep, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				continue
			}
			if !deploymentReady(dep) {
				continue
			}
			image := primaryImage(dep)
			log.Printf("deploy: %s image=%s", dep.Name, image)
			if err := k.sendEvent("deploy", dep.Name, map[string]any{
				"version":   image,
				"namespace": dep.Namespace,
				"replicas":  dep.Status.ReadyReplicas,
			}); err != nil {
				log.Printf("error sending deploy event for %s: %v", dep.Name, err)
			}
		}
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
				log.Printf("alert: CrashLoopBackOff pod=%s container=%s", pod.Name, container)
				if err := k.sendEvent("alert", pod.Labels["app"], map[string]any{
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

func deploymentReady(dep *appsv1.Deployment) bool {
	return dep.Status.ReadyReplicas > 0 &&
		dep.Status.ReadyReplicas == dep.Status.Replicas &&
		dep.Status.UnavailableReplicas == 0
}

func primaryImage(dep *appsv1.Deployment) string {
	if len(dep.Spec.Template.Spec.Containers) == 0 {
		return "unknown"
	}
	return dep.Spec.Template.Spec.Containers[0].Image
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
