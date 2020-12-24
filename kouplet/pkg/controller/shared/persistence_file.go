package shared

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// PersistenceStore ...
type PersistenceStore interface {
	ReadValues(keys []string) (map[string]*string, error)
	WriteValues(values map[string]string) error
	ListKeysByPrefix(prefix string) ([]string, error)
	DeleteByKey(keys []string) error
}

// FilePersistence ...
type FilePersistence struct {
	outputPath string
}

// FilePersistence should implement the store interface
var _ PersistenceStore = &FilePersistence{}

func NewFilePersistence(path string) (*FilePersistence, error) {

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)

		if err != nil {
			return nil, err
		}
	}

	return &FilePersistence{outputPath: path}, nil
}

func (fp *FilePersistence) ListKeysByPrefix(prefix string) ([]string, error) {
	result := []string{}

	files, err := ioutil.ReadDir(fp.outputPath)
	if err != nil {
		return result, err
	}

	for _, file := range files {

		name := file.Name()

		noTxtName := strings.ReplaceAll(name, ".txt", "")

		if strings.HasPrefix(noTxtName, prefix) {

			result = append(result, noTxtName)
		}

	}

	return result, nil

}

func (fp *FilePersistence) DeleteByKey(keys []string) error {

	for _, key := range keys {
		outputPath := path.Join(fp.outputPath, key+".txt")

		err := os.Remove(outputPath)
		if err != nil {
			LogErrorMsg(err, "Unable to delete key: "+key)
			return err
		}
	}

	return nil
}

func (fp *FilePersistence) ReadValues(keys []string) (map[string]*string, error) {

	result := make(map[string]*string)

	for _, key := range keys {
		inputPath := path.Join(fp.outputPath, key+".txt")

		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			LogDebugErr(errors.New("Unable to find requested file " + inputPath))
			result[inputPath] = nil
			continue
		}

		val, err := ioutil.ReadFile(inputPath)
		if err != nil {
			LogSevereMsg(err, "Unable to read file: "+inputPath)
			return nil, err
		}

		str := string(val)
		result[key] = &str

	}

	return result, nil

}

func (fp *FilePersistence) WriteValues(values map[string]string) error {

	for key, value := range values {

		outputPath := path.Join(fp.outputPath, key+".txt")

		file, err := os.Create(outputPath)

		if err != nil || file == nil {
			LogErrorMsg(err, "Error when writing values: "+key)
			return err
		}

		defer file.Close()

		_, err = file.WriteString(value)
		if err != nil || file == nil {
			LogErrorMsg(err, "Error when writing values: "+key)
			return err
		}

		file.Sync()
	}

	return nil
}
