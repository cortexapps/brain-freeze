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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"strconv"
	"strings"

	"path/filepath"
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

	logger.Info().Msg("Running k8s dump command")
	logger.Info().Msg("kubeconfig is : " + kubeconfig)
	logger.Info().Msg("namespace is : " + namespace)
	logger.Info().Msg("helmDeployment is : " + helmDeployment)

	clientset := getClientSet(kubeconfig)

	helmClient, err := helmclient.New(&helmclient.Options{})
	if err != nil {
		logger.Error().Msg("Error while creating helm client: " + err.Error())
	}
	helmRelease, err := helmClient.GetRelease(helmDeployment)
	if err != nil {
		logger.Error().Msg("Error while fetching helm release: " + err.Error())
	}
	utils.WriteToFile("data/helm/helm-release-manifest.yaml", helmRelease.Manifest)

	releaseValues, err := helmClient.GetReleaseValues(helmDeployment, true)
	jsonString, err := json.Marshal(releaseValues)
	if err != nil {
		logger.Error().Msg("Error while marshalling helm release values: " + err.Error())
	}
	utils.WriteToFile("data/helm/values.json", string(jsonString))

	dumpDeployments(*clientset, namespace)
	dumpPods(*clientset, namespace)
	dumpConfigMaps(*clientset, namespace)
	dumpServices(*clientset, namespace)
	dumpSecrets(*clientset, namespace)
}

func runLogsCommand(kubeconfig string, namespace string, timesinceseconds int64) {
	logger := utils.GetLogger()

	logger.Info().Msg("Running k8s dump command")
	logger.Info().Msg("kubeconfig is : " + kubeconfig)
	logger.Info().Msg("namespace is : " + namespace)

	clientset := getClientSet(kubeconfig)

	dumpPodLogs(*clientset, namespace, timesinceseconds)
}

func getClientSet(kubeconfig string) *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return clientset
}

func dumpDeployments(clientset kubernetes.Clientset, namespace string) {
	logger := utils.GetLogger()
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	logger.Info().Msg("Listing deployments in namespace " + namespace)
	list, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing deployments: " + err.Error())
	}
	for _, d := range list.Items {
		fmt.Printf(" * %s (%d replicas)\n", d.Name, *d.Spec.Replicas)
		desriber := utils.DeploymentDescriber{
			Client: &clientset,
		}
		res, err := desriber.Describe(namespace, d.Name, utils.DescriberSettings{ChunkSize: 128})
		if err != nil {
			logger.Error().Msg("Error while describing deployment: " + err.Error())
		}

		fmt.Printf(" * DESCRIBE Deployment:: %s \n\n\n\n ", res)
		utils.WriteToFile("data/deployments/"+d.Name+".yaml", res)
	}
}

func dumpPods(clientset kubernetes.Clientset, namespace string) {
	logger := utils.GetLogger()
	podsClient := clientset.CoreV1().Pods(namespace)
	pods, err := podsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing pods: " + err.Error())
	}
	for _, d := range pods.Items {
		fmt.Printf(" * %s \n", d.Name)
		fmt.Printf(" * NAME:: %s \n\n\n\n ", d.Name)
		describer := utils.PodDescriber{&clientset}
		res, err := describer.Describe(namespace, d.Name, utils.DescriberSettings{ShowEvents: true})
		if err != nil {
			logger.Error().Msg("Error while describing pod: " + err.Error())
		}

		fmt.Printf(" * DESCRIBE Pod:: %s \n\n\n\n ", res)
		utils.WriteToFile("data/pods/"+d.Name+".yaml", res)
	}
}

func dumpPodLogs(clientset kubernetes.Clientset, namespace string, timesinceseconds int64) {
	logger := utils.GetLogger()
	podsClient := clientset.CoreV1().Pods(namespace)
	pods, err := podsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing pods: " + err.Error())
	}
	for _, d := range pods.Items {
		logger.Info().Msg("Getting logs for pod: " + d.Name + " for seconds: " + strconv.FormatInt(timesinceseconds, 10))
		utils.WriteToFile("data/logs/"+d.Name+".log", getPodLogs(clientset, d, timesinceseconds))
	}
}

func dumpConfigMaps(clientset kubernetes.Clientset, namespace string) {
	logger := utils.GetLogger()
	configMapsClient := clientset.CoreV1().ConfigMaps(namespace)
	configMaps, err := configMapsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing configmaps: " + err.Error())
	}
	for _, d := range configMaps.Items {
		fmt.Printf(" * %s \n", d.Name)
		fmt.Printf(" * NAME:: %s \n\n\n\n ", d.Name)
		describer := utils.ConfigMapDescriber{&clientset}
		res, err := describer.Describe(namespace, d.Name, utils.DescriberSettings{ChunkSize: 128})
		if err != nil {
			logger.Error().Msg("Error while describing configmap: " + err.Error())
		}

		fmt.Printf(" * DESCRIBE ConfigMap:: %s \n\n\n\n ", res)
		utils.WriteToFile("data/configmaps/"+d.Name+".yaml", res)
	}
}

func dumpServices(clientset kubernetes.Clientset, namespace string) {
	logger := utils.GetLogger()
	servicesClient := clientset.CoreV1().Services(namespace)
	services, err := servicesClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing services: " + err.Error())
	}
	for _, d := range services.Items {
		fmt.Printf(" * %s \n", d.Name)
		fmt.Printf(" * NAME:: %s \n\n\n\n ", d.Name)
		describer := utils.ServiceDescriber{&clientset}
		res, err := describer.Describe(namespace, d.Name, utils.DescriberSettings{ChunkSize: 128})
		if err != nil {
			logger.Error().Msg("Error while describing service: " + err.Error())
		}

		fmt.Printf(" * DESCRIBE Service:: %s \n\n\n\n ", res)
		utils.WriteToFile("data/services/"+d.Name+".yaml", res)
	}
}

func dumpSecrets(clientset kubernetes.Clientset, namespace string) {
	logger := utils.GetLogger()
	secretsClient := clientset.CoreV1().Secrets(namespace)
	secrets, err := secretsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Error().Msg("Error while listing secrets: " + err.Error())
	}
	for _, d := range secrets.Items {

		if !strings.HasPrefix(d.Name, "sh.helm") {
			fmt.Printf(" * %s \n", d.Name)
			fmt.Printf(" * NAME:: %s \n\n\n\n ", d.Name)
			describer := utils.SecretDescriber{&clientset}
			res, err := describer.Describe(namespace, d.Name, utils.DescriberSettings{ChunkSize: 128})
			if err != nil {
				logger.Error().Msg("Error while describing secret: " + err.Error())
			}

			fmt.Printf(" * DESCRIBE Secret:: %s \n\n\n\n ", res)
			utils.WriteToFile("data/secrets/"+d.Name+".yaml", res)
		}
	}
}

func getPodLogs(clientSet kubernetes.Clientset, pod apiv1.Pod, timesinceseconds int64) string {
	podLogOpts := apiv1.PodLogOptions{
		SinceSeconds: &timesinceseconds,
	}
	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "error in opening stream"
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "error in copy information from podLogs to buf"
	}
	str := buf.String()

	return str
}
