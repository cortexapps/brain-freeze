package utils

import "os"

// TODO Refactor this and break into smaller functions
func CreateDirectory(path string) {
	logger := GetLogger()
	err := os.MkdirAll(path, 0700)
	if err != nil {
		logger.Error().Msg("Error while creating directory: " + err.Error())
	}
}
func WriteToFile(fileName string, data string) {
	logger := GetLogger()
	logger.Info().Msg("Creating file: " + fileName)
	out, e := os.Create(fileName)
	if e != nil {
		logger.Error().Msg("Error while creating file: " + e.Error())
	}

	_, err := out.WriteString(data + "\n")
	if err != nil {
		logger.Error().Msg("Error while writing to file: " + err.Error())
	}
}
