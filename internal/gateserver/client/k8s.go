package client

import (
	"context"
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Config struct {
	Host  string
	Token string
	Port  int
}

var config *Config

func queryAllPods(ctx context.Context) (*v1.PodList, error) {
	clientSet, _ := NewKubernetesClient(config)
	return clientSet.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
}

func createPod() (*v1.Pod, error) {
	return nil, nil
}

func NewKubernetesClient(c *Config) (*kubernetes.Clientset, error) {
	kubeConf := &rest.Config{
		Host:        fmt.Sprintf("%s:%d", c.Host, c.Port),
		BearerToken: c.Token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	return kubernetes.NewForConfig(kubeConf)
}
