package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	ctx := context.Background()

	cfg, err := BuildConfig()
	if err != nil {
		log.Fatalf("failed to build kubeconfig: %v", err)
	}

	// Read allowed node label keys from environment (comma-separated).
	// If empty, controller applies to all nodes.
	allowlist := os.Getenv("NODE_LABEL_ALLOWLIST")
	var allowedKeys []string
	if allowlist != "" {
		for _, k := range strings.Split(allowlist, ",") {
			if t := strings.TrimSpace(k); t != "" {
				allowedKeys = append(allowedKeys, t)
			}
		}
	}

	c, err := NewNodeLifeSupportController(cfg, allowedKeys)
	if err != nil {
		log.Fatalf("failed to init controller: %v", err)
	}

	log.Printf("node-life-support controller starting…")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		if err := c.SyncAllNodes(ctx); err != nil {
			log.Printf("sync error: %v", err)
		}
		<-ticker.C
	}
}

func BuildConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

type NodeLifeSupportController struct {
	client        *kubernetes.Clientset
	allowedLabels map[string]struct{}
}

func NewNodeLifeSupportController(cfg *rest.Config, allowedKeys []string) (*NodeLifeSupportController, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct{})
	for _, k := range allowedKeys {
		if k != "" {
			m[k] = struct{}{}
		}
	}
	return &NodeLifeSupportController{client: client, allowedLabels: m}, nil
}

func (c *NodeLifeSupportController) SyncAllNodes(ctx context.Context) error {
	nodes, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	for _, n := range nodes.Items {
		// If allowedLabels is non-empty, only operate on nodes that have any of the allowed label keys.
		if len(c.allowedLabels) > 0 {
			if !c.nodeHasAllowedLabel(&n) {
				log.Printf("skipping node %s: no matching allowed labels", n.Name)
				continue
			}
		}

		if err := c.SyncNode(ctx, &n); err != nil {
			log.Printf("failed updating node %s: %v", n.Name, err)
		} else {
			log.Printf("updated node %s", n.Name)
		}
	}

	return nil
}

func (c *NodeLifeSupportController) SyncNode(ctx context.Context, node *v1.Node) error {
	if err := c.UpdateLease(ctx, node.Name); err != nil {
		return fmt.Errorf("update lease: %w", err)
	}

	if err := c.ForceNodeReady(ctx, node.Name); err != nil {
		return fmt.Errorf("update node status: %w", err)
	}

	return nil
}

func (c *NodeLifeSupportController) UpdateLease(ctx context.Context, nodeName string) error {
	leaseName := nodeName
	// Kubernetes expects timestamps with microsecond precision (6 fractional digits).
	// Format time accordingly to avoid parsing errors when the API server decodes the patch.
	now := time.Now().UTC()
	renew := now.Format("2006-01-02T15:04:05.000000Z07:00")

	patch := fmt.Sprintf(`{
			"spec": {
				"holderIdentity": %q,
				"renewTime": %q
			}
		}`, nodeName, renew)

	_, err := c.client.CoordinationV1().Leases("kube-node-lease").Patch(
		ctx,
		leaseName,
		types.MergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
	)

	return err
}

func (c *NodeLifeSupportController) ForceNodeReady(ctx context.Context, nodeName string) error {
	ready := v1.NodeCondition{
		Type:               v1.NodeReady,
		Status:             v1.ConditionTrue,
		LastHeartbeatTime:  metav1.Time{Time: time.Now().UTC()},
		LastTransitionTime: metav1.Time{Time: time.Now().UTC()},
		Reason:             "NodeLifeSupportOverride",
		Message:            "node-life-support controller asserting node health.",
	}

	patchObj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []v1.NodeCondition{ready},
		},
	}

	raw, err := json.Marshal(patchObj)
	if err != nil {
		return err
	}

	_, err = c.client.CoreV1().Nodes().Patch(
		ctx,
		nodeName,
		types.MergePatchType,
		raw,
		metav1.PatchOptions{},
		"status",
	)

	return err
}

// nodeHasAllowedLabel returns true if the node has at least one label key
// that exists in the controller's allowedLabels set.
func (c *NodeLifeSupportController) nodeHasAllowedLabel(node *v1.Node) bool {
	if node == nil {
		return false
	}
	if len(c.allowedLabels) == 0 {
		return true
	}
	for k := range node.Labels {
		if _, ok := c.allowedLabels[k]; ok {
			return true
		}
	}
	return false
}
