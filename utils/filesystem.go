package utils

import "os"

func WriteToFile(fileName string, data string) {
	logger := GetLogger()
	logger.Info().Msg("Creating file: " + fileName)
	err := os.MkdirAll("data", 0700)
	if err != nil {
		logger.Error().Msg("Error while creating directory: " + err.Error())
	}

	err = os.MkdirAll("data/helm", 0700)
	if err != nil {
		logger.Error().Msg("Error while creating data/helm: " + err.Error())
	}
	err = os.MkdirAll("data/actuator", 0700)
	if err != nil {
		logger.Error().Msg("Error while creating data/helm: " + err.Error())
	}
	os.MkdirAll("data/deployments", 0700)
	os.MkdirAll("data/pods", 0700)
	os.MkdirAll("data/secrets", 0700)
	os.MkdirAll("data/services", 0700)
	os.MkdirAll("data/configmaps", 0700)
	os.MkdirAll("data/logs", 0700)

	out, e := os.Create(fileName)
	if e != nil {
		logger.Error().Msg("Error while creating file: " + e.Error())
	}

	_, err = out.WriteString(data + "\n")
	if err != nil {
		logger.Error().Msg("Error while writing to file: " + err.Error())
	}
}
