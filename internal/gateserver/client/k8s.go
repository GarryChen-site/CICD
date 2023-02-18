package client

import (
	"context"
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

const (
	Mem_Oversubscribe_Flag = 1 << 0
)

var env string

var k8sApiServer string

var k8sBearerToken string

type Config struct {
	Host  string
	Token string
	Port  int
}

type PodHttpReadinessProbe struct {
	path                string
	port                int
	failureThreshold    int32
	initialDelaySeconds int32
	periodSeconds       int32
	timeoutSeconds      int32
	successThreshold    int32
}

func newProbe() PodHttpReadinessProbe {
	probe := PodHttpReadinessProbe{}
	probe.path = "/hs"
	probe.port = 8080
	probe.failureThreshold = 12
	probe.initialDelaySeconds = 60
	probe.periodSeconds = 5
	probe.timeoutSeconds = 3
	probe.successThreshold = 3
	return probe
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

	podSpec.Volumes = createVolumes()

	reference := v1.LocalObjectReference{Name: "dockeryardkey"}
	podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets, reference)

	var containers []v1.Container
	containers = append(containers, createContainer(template))

	podSpec.Containers = containers
	podSpec.RestartPolicy = "Always"

	var v1Sysctls []v1.Sysctl
	splits := strings.Split(template.Sysctl, ",")
	for _, s := range splits {
		v1Sysctl := v1.Sysctl{}
		pair := strings.Split(strings.TrimSpace(s), "=")
		if len(pair) > 1 {
			v1Sysctl.Name = pair[0]
			v1Sysctl.Value = pair[1]
			v1Sysctls = append(v1Sysctls, v1Sysctl)
		}
	}

	securityContext := &v1.PodSecurityContext{}
	securityContext.Sysctls = v1Sysctls
	podSpec.SecurityContext = securityContext

	var hostAliases []v1.HostAlias
	v1HostAlias := v1.HostAlias{}
	v1HostAlias.IP = "127.0.0.1"
	var hostNames1 []string
	hostNames1 = append(hostNames1, "localhost.localdomain", "localhost4", "localhost4.localdomain4")
	v1HostAlias.Hostnames = hostNames1
	hostAliases = append(hostAliases, v1HostAlias)

	var hostNames2 []string
	hostNames1 = append(hostNames2, "localhost.localdomain", "localhost6", "localhost6.localdomain6")
	v1HostAlias.Hostnames = hostNames2
	hostAliases = append(hostAliases, v1HostAlias)

	podSpec.HostAliases = hostAliases

	pod.Spec = podSpec

	return pod
}

func createVolumes() []v1.Volume {
	var v1Volumes []v1.Volume

	v1Volume := v1.Volume{}
	v1Volume.Name = "cpuinfo"
	v1HostPathVolumeSource := &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/cpuinfo"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "diskstats"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/diskstats"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "meminfo"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/meminfo"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "stat"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/stat"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "swaps"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/swaps"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "uptime"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/var/lib/lxcfs/proc/uptime"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	v1Volume = v1.Volume{}
	v1Volume.Name = "localtime"
	v1HostPathVolumeSource = &v1.HostPathVolumeSource{}
	v1HostPathVolumeSource.Path = "/usr/share/zoneinfo/Asia/Shanghai"
	v1Volume.HostPath = v1HostPathVolumeSource
	v1Volumes = append(v1Volumes, v1Volume)

	return v1Volumes
}

func createContainer(template AppPodTemplate) v1.Container {
	v1Container := v1.Container{}

	var envs []v1.EnvVar

	envVar := v1.EnvVar{}
	envVar.Name = "APP_ID"
	envVar.Value = template.AppID
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "APP_NAME"
	envVar.Value = template.AppName
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "INSTANCE_NAME"
	envVar.Value = template.PodName
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "ENV"
	if strings.HasPrefix(env, "fat") || strings.HasPrefix(env, "lpt") {
		envVar.Value = "fat"
	} else if strings.HasPrefix(env, "uat") {
		envVar.Value = "uat"
	} else {
		envVar.Value = env
	}
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "TZ"
	envVar.Value = "Asia/Shanghai"
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "LANG"
	envVar.Value = "en_US.UTF-8"
	envs = append(envs, envVar)

	envVar = v1.EnvVar{}
	envVar.Name = "LC_ALL"
	envVar.Value = "en_US.UTF-8"
	envs = append(envs, envVar)

	// todo 不知道具体格式
	//envVars := template.EnvJson

	var javaOpts strings.Builder
	if len(template.K8sQuota.GetJavaOpts()) > 0 {
		javaOpts.WriteString(template.K8sQuota.GetJavaOpts())
	}
	if len(javaOpts.String()) > 0 {
		envVar = v1.EnvVar{}
		envVar.Name = "JAVA_TOOLS_OPTIONS"
		envVar.Value = javaOpts.String()
		envs = append(envs, envVar)
	}

	v1Container.Env = envs

	requirements := v1.ResourceRequirements{}
	if strings.EqualFold(env, "pro") {
		limits := v1.ResourceList{}
		limits["memory"] = template.K8sQuota.GetLimitMemory()
		limits["cpu"] = template.K8sQuota.GetLimitCPU()
		requirements.Limits = limits

		requests := v1.ResourceList{}
		requests["cpu"] = template.K8sQuota.GetRequestCPU()
		requirements.Requests = requests

		if template.Flags&Mem_Oversubscribe_Flag != 0 {
			requirements.Requests["memory"] = template.K8sQuota.GetRequestMemory()
		} else {
			requirements.Requests["memory"] = template.K8sQuota.GetLimitMemory()
		}
	} else {
		limits := v1.ResourceList{}
		limits["memory"] = template.K8sQuota.GetLimitMemory()
		limits["cpu"] = template.K8sQuota.GetLimitCPU()
		requirements.Limits = limits

		requests := v1.ResourceList{}
		requests["cpu"] = template.K8sQuota.GetRequestCPU()
		requests["memory"] = template.K8sQuota.GetRequestMemory()
		requirements.Requests = requests
	}

	v1Container.Resources = requirements
	v1Container.Image = template.Image
	v1Container.ImagePullPolicy = "IfNotPresent"
	v1Container.Name = formatContainerName(template.AppName)
	v1Container.VolumeMounts = createVolumeMounts()

	v1Container.ReadinessProbe = createReadinessProbe(newProbe())

	return v1Container
}

func formatContainerName(containerName string) string {
	return strings.ReplaceAll(containerName, "\\.", "-")
}

func createReadinessProbe(probe PodHttpReadinessProbe) *v1.Probe {

	v1Probe := &v1.Probe{}

	httpGetAction := &v1.HTTPGetAction{}
	httpGetAction.Path = probe.path
	httpGetAction.Port = intstr.FromInt(probe.port)
	v1Probe.HTTPGet = httpGetAction

	v1Probe.FailureThreshold = probe.failureThreshold
	v1Probe.InitialDelaySeconds = probe.initialDelaySeconds
	v1Probe.PeriodSeconds = probe.periodSeconds
	v1Probe.TimeoutSeconds = probe.timeoutSeconds
	v1Probe.SuccessThreshold = probe.successThreshold

	return v1Probe
}

func createVolumeMounts() []v1.VolumeMount {
	var volumeMounts []v1.VolumeMount

	v1VolumeMount := v1.VolumeMount{}
	v1VolumeMount.Name = "cpuinfo"
	v1VolumeMount.MountPath = "/proc/cpuinfo"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "diskstats"
	v1VolumeMount.MountPath = "/proc/diskstats"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "meminfo"
	v1VolumeMount.MountPath = "/proc/meminfo"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "stat"
	v1VolumeMount.MountPath = "/proc/stat"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "swaps"
	v1VolumeMount.MountPath = "/proc/swaps"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "uptime"
	v1VolumeMount.MountPath = "/proc/uptime"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	v1VolumeMount = v1.VolumeMount{}
	v1VolumeMount.Name = "localtime"
	v1VolumeMount.MountPath = "/etc/localtime"
	volumeMounts = append(volumeMounts, v1VolumeMount)

	return volumeMounts
}

func CreateNamespace(namespace string) error {

	return nil
}
