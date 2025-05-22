/*
Copyright © 2024 Cortex <aditya.bansal@cortex.io>
*/
package cmd

import (
	"brain-freeze/utils"
	"bytes"
	"context"
	"encoding/json"
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
	logger := utils.GetLogger()
	helmClient, err := helmclient.New(&helmclient.Options{
		Namespace: namespace,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Error creating helm client")
	} else {
		helmRelease, err := helmClient.GetRelease(helmDeployment)
		if err != nil {
			logger.Error().Err(err).Msgf("Error fetching helm release %s", helmDeployment)
		} else {
			utils.WriteToFile("data/helm/manifest.yaml", helmRelease.Manifest)
		}

		releaseValues, err := helmClient.GetReleaseValues(helmDeployment, true)
		if err != nil {
			logger.Error().Err(err).Msgf("Error getting values for helm release %s", helmDeployment)
		} else {
			jsonString, err := json.Marshal(releaseValues)
			if err != nil {
				logger.Fatal().Err(err).Msg("Error while marshalling helm release values")
			} else {
				utils.WriteToFile("data/helm/values.json", string(jsonString))
			}
		}
	}

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

type ResourceDumpOptions struct {
	IncludeByName func(string) bool
}

func dumpResources(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace string, opts ResourceDumpOptions) {
	logger := utils.GetLogger().With().Str("resource", gvr.String()).Logger()

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
