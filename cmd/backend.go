/*
Copyright © 2024 Cortex <aditya.bansal@cortex.io>
*/
package cmd

import (
	"brain-freeze/utils"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
)

// backendCmd represents the backend command
var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Commands to fetch data from the backend",
}

var infoCommand = &cobra.Command{
	Use:   "info",
	Short: "Fetch all the debug info that is exposed to actuator/info",
	Run: func(cmd *cobra.Command, args []string) {
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		runInfoCmd(url, token)
	},
}

func init() {
	rootCmd.AddCommand(backendCmd)
	backendCmd.AddCommand(infoCommand)

	backendCmd.PersistentFlags().String("url", "http://app.helm.getcortexapp.com", "(required) absolute path to the kubeconfig file")
	backendCmd.PersistentFlags().String("token", "", "(required) absolute path to the kubeconfig file")
}

func runInfoCmd(url string, token string) {
	logger := utils.GetLogger()
	logger.Info().Msg("Running info command with URL: " + url)

	jsonActuator := getActuatorInf(url, token)
	utils.WriteToFile("data/actuator/info.json", string(jsonActuator))
}

func getActuatorInf(host string, token string) string {
	logger := utils.GetLogger()
	client := http.Client{}
	req, err := http.NewRequest("GET", host+"/actuator/info", nil)
	if err != nil {
		logger.Error().Msg("Error creating request + " + err.Error())
	}

	req.Header = http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer " + token},
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Msg("Error making request + " + err.Error())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error().Msg("Error reading response body + " + err.Error())
	}
	sb := string(body)
	return sb
}
