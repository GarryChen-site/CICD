package client

import (
	"context"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

type Config struct {
	Host  string
	Token string
	Port  int
}

type K8sQuota interface {
	// CPU 配额
	GetRequestCPU() resource.Quantity

	GetRequestMemory() resource.Quantity

	GetLimitCPU() resource.Quantity

	GetLimitMemory() resource.Quantity

	// 告诉k8s需要从该scope扣除资源
	GetScope() string

	GetJavaOpts() string
}

type AppPodTemplate struct {
	Namespace string
	AppID     string
	AppName   string
	Image     string
	K8sQuota  K8sQuota
	Dns       string
	PodName   string
	PodIP     string
	Port      int32
	EnvJson   string
	Sysctl    string
	Flags     int64
}

var config *Config

func queryAllPods(ctx context.Context) (*v1.PodList, error) {
	clientSet, _ := NewKubernetesClient(config)
	return clientSet.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
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

func createPod(template AppPodTemplate) *v1.Pod {
	pod := &v1.Pod{}
	pod.APIVersion = "v1"
	pod.Kind = "Pod"

	objectMeta := metav1.ObjectMeta{}
	objectMeta.Name = template.PodName

	labels := make(map[string]string)
	labels["app"] = template.AppName
	labels["appid"] = template.AppID
	labels["instance"] = template.PodName
	labels["ip"] = template.PodIP

	objectMeta.Labels = labels
	pod.ObjectMeta = objectMeta

	podSpec := v1.PodSpec{}
	podSpec.Hostname = strings.ReplaceAll(template.PodName, "\\.", "")
	podSpec.PriorityClassName = template.K8sQuota.GetScope()

	if len(template.Dns) == 0 {
		podSpec.DNSPolicy = "Default"
	} else {
		podSpec.DNSPolicy = "None"
		dnsConfig := &v1.PodDNSConfig{}
		dnsConfig.Nameservers = []string{template.Dns}
		podSpec.DNSConfig = dnsConfig
	}

	return pod
}
