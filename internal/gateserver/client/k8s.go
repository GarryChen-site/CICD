package client

import (
	"bytes"
	"context"
	"github.com/google/martian/log"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"strings"
	"unicode"
)

const (
	Mem_Oversubscribe_Flag = 1 << 0
)

//var env string

type Conf struct {
	K8sApiServer string

	K8sBearerToken string

	Env string

	RestConf *rest.Config
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

func (c *Conf) QueryAllPods(ctx context.Context) (*v1.PodList, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
}

func (c *Conf) QueryAllPodsWithLabel(ctx context.Context, labelSelectorMap map[string]string) (*v1.PodList, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return nil, err
	}
	labelSelect := convert(labelSelectorMap)

	opts := metav1.ListOptions{}
	opts.LabelSelector = labelSelect

	return clientset.CoreV1().Pods(v1.NamespaceAll).List(ctx, opts)
}

func (c *Conf) QueryAppPods(ctx context.Context, namespace string, labelSelectorMap map[string]string) (*v1.PodList, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return nil, err
	}
	labelSelect := convert(labelSelectorMap)

	opts := metav1.ListOptions{}
	opts.LabelSelector = labelSelect

	return clientset.CoreV1().Pods(namespace).List(ctx, opts)
}

func convert(labelSelectorMap map[string]string) string {
	var sb strings.Builder
	for key, value := range labelSelectorMap {
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(value)
		sb.WriteString(",")
	}
	labelSelect := sb.String()
	if len(labelSelect) > 0 {
		labelSelect = labelSelect[:len(labelSelect)-1]
	}
	return labelSelect
}

func (c *Conf) DeployAppPod(ctx context.Context, temp *AppPodTemplate) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	pod := c.createPod(temp)
	result, podCreateErr := clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if podCreateErr != nil {
		return podCreateErr
	}
	log.Infof("Pod created,result %s", result.String())
	return nil
}

func (c *Conf) UpdateAppPod(ctx context.Context, dockerURL, namespace, podName, image string) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	return nil
}

func (c *Conf) DeleteAppPod(ctx context.Context, namespace, podName string) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationBackground
	dele := metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	deleteErr := clientset.CoreV1().Pods(namespace).Delete(ctx, podName, dele)
	if deleteErr != nil {
		log.Infof("Pod deleted failed,instanceName:%s,err:%v", podName, deleteErr)
		return deleteErr
	}
	log.Infof("Pod deleted successfully,instanceName:%s", podName)
	return nil
}

func (c *Conf) ForceDeleteAppPod(ctx context.Context, namespace, podName string) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationBackground
	var time *int64
	dele := metav1.DeleteOptions{PropagationPolicy: &propagationPolicy, GracePeriodSeconds: time}
	deleteErr := clientset.CoreV1().Pods(namespace).Delete(ctx, podName, dele)
	if deleteErr != nil {
		log.Infof("Pod deleted failed,instanceName:%s,err:%v", podName, deleteErr)
		return deleteErr
	}
	log.Infof("Pod deleted successfully,instanceName:%s", podName)
	return nil
}

func NewKubernetesConf(env, k8sApiServer, k8sBearerToken string) *Conf {
	r := &rest.Config{
		APIPath:     k8sApiServer,
		BearerToken: k8sBearerToken,
	}

	c := &Conf{
		Env:      env,
		RestConf: r,
	}
	return c
}

func (c *Conf) createPod(template *AppPodTemplate) *v1.Pod {
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
	containers = append(containers, c.createContainer(template))

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

func (c *Conf) createContainer(template *AppPodTemplate) v1.Container {
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
	if strings.HasPrefix(c.Env, "fat") || strings.HasPrefix(c.Env, "lpt") {
		envVar.Value = "fat"
	} else if strings.HasPrefix(c.Env, "uat") {
		envVar.Value = "uat"
	} else {
		envVar.Value = c.Env
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
	if strings.EqualFold(c.Env, "pro") {
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

func (c *Conf) CreateNamespace(ctx context.Context, namespace string) error {

	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}

	nsName := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
	}

	_, err = clientset.CoreV1().Namespaces().Create(ctx, nsName, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Conf) CreateConfigMap(ctx context.Context, namespace string, configName string, dataMap map[string]string) error {

	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}

	body := &v1.ConfigMap{}
	body.APIVersion = "v1"
	body.Data = dataMap

	objectMeta := metav1.ObjectMeta{Name: configName}
	body.ObjectMeta = objectMeta

	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, body, metav1.CreateOptions{})
	if err != nil {
		return nil
	}
	return nil
}

func (c *Conf) GetConfigMap(ctx context.Context, namespace string, configName string) (*v1.ConfigMap, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return nil, err
	}
	return clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configName, metav1.GetOptions{})
}

func (c *Conf) DeleteConfigMap(ctx context.Context, namespace string, configName string) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationBackground
	var time *int64
	dele := metav1.DeleteOptions{PropagationPolicy: &propagationPolicy, GracePeriodSeconds: time}
	deleteErr := clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configName, dele)
	if deleteErr != nil {
		return deleteErr
	}
	return nil
}

func (c *Conf) ExecCommand(ctx context.Context, pod string, namespace string, commands []string) (string, string, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return "", "", err
	}
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	req := clientset.CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: commands,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(c.RestConf, "POST", req.URL())
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", err
	}
	return buf.String(), errBuf.String(), nil
}

func (c *Conf) GetNodeByIP(ctx context.Context, hostIP string) (*v1.Node, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return nil, err
	}
	opts := metav1.ListOptions{
		LabelSelector: "kubernetes.io/hostname=" + hostIP,
		Limit:         1,
	}
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(nodeList.Items) > 0 {
		return &nodeList.Items[0], nil
	}
	return nil, nil
}

func (c *Conf) GetAppPodLog(ctx context.Context, namespace, instanceName string) (string, error) {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return "", err
	}
	opts := &v1.PodLogOptions{}
	opts.Follow = false
	limitBytes := int64(1 * 1024 * 1024)
	opts.LimitBytes = &limitBytes
	opts.Previous = false
	opts.SinceSeconds = nil
	tailLines := int64(5000)
	opts.TailLines = &tailLines
	opts.Timestamps = false
	req := clientset.CoreV1().Pods(namespace).GetLogs(instanceName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	// Read log output
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err != io.EOF {
				panic(err)
			}
			break
		}
		return string(buf[:n]), nil
	}
	return "", nil
}

func (c *Conf) CreateService(ctx context.Context, appName, namespace string) error {
	clientset, err := kubernetes.NewForConfig(c.RestConf)
	if err != nil {
		return err
	}
	service := &v1.Service{}
	objectMeta := metav1.ObjectMeta{}
	objectMeta.Name = getServiceFromAppName(appName)
	service.ObjectMeta = objectMeta

	service.Kind = "Service"
	service.APIVersion = "v1"

	spec := v1.ServiceSpec{}
	selector := map[string]string{}
	selector["app"] = appName
	spec.Selector = selector

	port := v1.ServicePort{}
	port.Protocol = v1.ProtocolTCP
	port.Port = 8080
	port.TargetPort = intstr.FromInt(8080)
	spec.Ports = []v1.ServicePort{port}

	service.Spec = spec

	_, err = clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func getServiceFromAppName(appName string) string {
	service := strings.ReplaceAll(appName, ".", "-")
	if unicode.IsDigit(rune(appName[0])) {
		return "s" + service
	}
	return service
}
