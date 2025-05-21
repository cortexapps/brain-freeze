package utils

import "os"

func CreateDirectory(path string) {
	logger := GetLogger()
	err := os.MkdirAll(path, 0700)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error while creating directory")
	}
}

func WriteToFile(fileName string, data string) {
	logger := GetLogger()
	logger.Info().Msgf("Creating file %s", fileName)
	out, e := os.Create(fileName)
	if e != nil {
		logger.Fatal().Err(e).Msgf("Error creating file %s", fileName)
	}

	_, err := out.WriteString(data + "\n")
	if err != nil {
		logger.Fatal().Err(err).Msgf("Error writing to file %s", fileName)
	}
}
