package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var client *kubernetes.Clientset
var ctx context.Context

func init() {
	ctx = context.Background()

	client = kubernetes.New(&rest.RESTClient{})

	return
}

func Scale(scale uint32) error {
	payload, _ := json.Marshal([]patchUint32Value{{
		Op:    "replace",
		Path:  "/spec/replicas",
		Value: scale,
	}})

	b, _ := json.Marshal(payload)
	sts, err := client.CoreV1().ReplicationControllers(namespace).Patch(ctx, "7tv-goapi", types.JSONPatchType, b, v1.PatchOptions{})
	fmt.Println("sts", sts)

	return err
}

func RolloutRestart() error {
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"app.7tv.restart.timestamp": time.Now().Format(time.Stamp),
					},
				},
			},
		},
	}

	b, _ := json.Marshal(patchData)
	_, err := client.AppsV1().StatefulSets(namespace).Patch(ctx, "7tv-goapi", types.MergePatchType, b, v1.PatchOptions{})
	if err != nil {
		log.Errorf("k8s, rollout restart, err=%v", err)
		return err
	}

	log.Info("k8s, rollout restart has started.")
	return nil
}

type patchUint32Value struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value uint32 `json:"value"`
}

var (
	namespace = "7tv"
)
