/*
Copyright © 2024 Cortex <aditya.bansal@cortex.io>
*/
package cmd

import (
	"brain-freeze/utils"
	"bytes"
	"context"
	"fmt"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"io"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
	"time"
)

// k8sCmd represents the k8s command
var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Commands to help debug the cortex k8s & helm installation",
}

var k8sDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump the entire k8s and helm installation info",
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		helmDeployment, _ := cmd.Flags().GetString("helm-deployment")

		runDumpCommand(kubeconfig, namespace, helmDeployment)
	},
}

var k8sLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Get logs from all the deployments in the namespace for the last n minutes",
	Run: func(cmd *cobra.Command, args []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		timeInMinutes, _ := cmd.Flags().GetInt64("timeInMinutes")

		runLogsCommand(kubeconfig, namespace, timeInMinutes*60)
	},
}

func init() {
	k8sCmd.AddCommand(k8sDumpCmd)
	k8sCmd.AddCommand(k8sLogsCmd)
	rootCmd.AddCommand(k8sCmd)

	home := homedir.HomeDir()
	k8sCmd.PersistentFlags().String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	k8sCmd.PersistentFlags().String("namespace", apiv1.NamespaceDefault, "(optional) The namespace of the k8s environment to use for the API")

	k8sDumpCmd.Flags().String("helm-deployment", "cortex-helm", "(optional) The name of the cortex helm-chart installation")
	k8sLogsCmd.Flags().Int64("timeInMinutes", 60, "(optional) Time (in minutes) to get the logs for")
}

func runDumpCommand(kubeconfig string, namespace string, helmDeployment string) {
	dumpHelm(namespace, helmDeployment)

	_, dynamicClient := getClients(kubeconfig)
	dumpDeployments(dynamicClient, namespace)
	dumpPods(dynamicClient, namespace)
	dumpConfigMaps(dynamicClient, namespace)
	dumpServices(dynamicClient, namespace)
	dumpSecrets(dynamicClient, namespace)
}

func runLogsCommand(kubeconfig string, namespace string, timesinceseconds int64) {
	clientset, _ := getClients(kubeconfig)
	dumpPodLogs(*clientset, namespace, timesinceseconds)
}

func getClients(kubeconfig string) (*kubernetes.Clientset, *dynamic.DynamicClient) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return clientset, dynamicClient
}

type HelmChartMetadata struct {
	Name       string
	Version    string
	AppVersion string
}

type HelmReleaseMetadata struct {
	Chart        HelmChartMetadata
	Dependencies []HelmChartMetadata
	DeployedAt   time.Time
	Name         string
	Namespace    string
	Revision     int
	Status       string
}

func dumpHelm(namespace string, releaseName string) {
	logger := utils.GetLogger().With().Str("helmRelease", releaseName).Str("namespace", namespace).Logger()
	client, err := helmclient.New(&helmclient.Options{
		Namespace: namespace,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Error creating helm client")
	} else {
		release, err := client.GetRelease(releaseName)
		if err != nil {
			logger.Error().Err(err).Msgf("Error fetching helm release %s", releaseName)
		} else {
			utils.WriteToFile("data/helm/manifest.yaml", release.Manifest)

			meta := HelmReleaseMetadata{
				Name:       release.Name,
				Namespace:  release.Namespace,
				Revision:   release.Version,
				Status:     release.Info.Status.String(),
				DeployedAt: release.Info.LastDeployed.Time,
				Chart: HelmChartMetadata{
					Name:       release.Chart.Name(),
					Version:    release.Chart.Metadata.Version,
					AppVersion: release.Chart.AppVersion(),
				},
			}

			metaYamlBytes, err := yaml.Marshal(meta)
			if err != nil {
				logger.Error().Err(err).Msg("Error marshalling helm release metadata")
			} else {
				utils.WriteToFile("data/helm/metadata.yaml", string(metaYamlBytes))
			}
		}

		releaseValues, err := client.GetReleaseValues(releaseName, true)
		if err != nil {
			logger.Error().Err(err).Msgf("Error getting values for helm release %s", releaseName)
		} else {
			jsonString, err := yaml.Marshal(releaseValues)
			if err != nil {
				logger.Fatal().Err(err).Msg("Error marshalling helm release values")
			} else {
				utils.WriteToFile("data/helm/values.yaml", string(jsonString))
			}
		}
	}
}

type ResourceDumpOptions struct {
	IncludeByName func(string) bool
}

func dumpResources(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace string, opts ResourceDumpOptions) {
	logger := utils.GetLogger().With().Str("resource", gvr.String()).Str("namespace", namespace).Logger()

	resourceClient := dynamicClient.Resource(gvr).Namespace(namespace)
	list, err := resourceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Err(err).Msgf("Error listing %s resources", gvr.String())
		return
	}

	for _, item := range list.Items {
		name := item.GetName()
		if opts.IncludeByName == nil || opts.IncludeByName(name) {
			fmt.Print("----------\n")
			fmt.Printf("%s: %s\n", gvr.String(), name)

			obj, err := resourceClient.Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				logger.Error().Err(err).Msgf("Error getting %s %s", gvr.String(), name)
				continue
			}

			yamlBytes, err := yaml.Marshal(obj)
			if err != nil {
				logger.Error().Err(err).Msgf("Error marshalling %s %s to YAML", gvr.String(), name)
				continue
			}

			fmt.Printf("%s\n", string(yamlBytes))

			filePath := filepath.Join("data", gvr.Resource, fmt.Sprintf("%s.yaml", name))
			utils.WriteToFile(filePath, string(yamlBytes))
		}
	}
}

func dumpDeployments(dynamicClient dynamic.Interface, namespace string) {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	dumpResources(dynamicClient, gvr, namespace, ResourceDumpOptions{})
}

func dumpPods(dynamicClient dynamic.Interface, namespace string) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	dumpResources(dynamicClient, gvr, namespace, ResourceDumpOptions{})
}

func dumpPodLogs(clientset kubernetes.Clientset, namespace string, timesinceseconds int64) {
	logger := utils.GetLogger()
	podsClient := clientset.CoreV1().Pods(namespace)
	pods, err := podsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("Error listing pods")
	}
	for _, p := range pods.Items {
		logger.Info().Msgf("Getting logs for pod %s for seconds %s", p.Name, strconv.FormatInt(timesinceseconds, 10))
		utils.WriteToFile("data/logs/"+p.Name+".log", getPodLogs(clientset, p, timesinceseconds))
	}
}

func getPodLogs(clientSet kubernetes.Clientset, pod apiv1.Pod, timesinceseconds int64) string {
	logger := utils.GetLogger().With().Str("pod", pod.Name).Logger()
	podLogOpts := apiv1.PodLogOptions{
		SinceSeconds: &timesinceseconds,
	}
	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "error in opening stream"
	}
	defer func() {
		if err := podLogs.Close(); err != nil {
			logger.Error().Err(err).Msgf("Error closing pod logs stream for %s", pod.Name)
		}
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from podLogs to buf"
	}
	str := buf.String()

	return str
}

func dumpConfigMaps(dynamicClient dynamic.Interface, namespace string) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	dumpResources(dynamicClient, gvr, namespace, ResourceDumpOptions{})
}

func dumpServices(dynamicClient dynamic.Interface, namespace string) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
	dumpResources(dynamicClient, gvr, namespace, ResourceDumpOptions{})
}

func dumpSecrets(dynamicClient dynamic.Interface, namespace string) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	opts := ResourceDumpOptions{
		IncludeByName: func(name string) bool {
			return !strings.HasPrefix(name, "sh.helm")
		},
	}

	dumpResources(dynamicClient, gvr, namespace, opts)
}
