package cmd

import (
	"brain-freeze/utils"
)

func CreateDataDirs() {
	utils.CreateDirectory("data")
	utils.CreateDirectory("data/helm")
	utils.CreateDirectory("data/actuator")
	utils.CreateDirectory("data/deployments")
	utils.CreateDirectory("data/pods")
	utils.CreateDirectory("data/secrets")
	utils.CreateDirectory("data/services")
	utils.CreateDirectory("data/configmaps")
	utils.CreateDirectory("data/logs")
}
