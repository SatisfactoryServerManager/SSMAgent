//go:build windows

package steamcmd

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

func ExtractArchive(file *os.File) error {
	utils.InfoLogger.Println("Extracting Steam CMD...")
	defer os.Remove(file.Name())

	archive, err := zip.OpenReader(file.Name())
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(SteamDir, f.Name)
		utils.DebugLogger.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(SteamDir)+string(os.PathSeparator)) {
			utils.DebugLogger.Println("invalid file path")
			return nil
		}
		if f.FileInfo().IsDir() {
			utils.DebugLogger.Println("creating directory...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}

	err = file.Close()
	if err != nil {
		return err
	}

	err = archive.Close()
	if err != nil {
		return err
	}

	err = os.Remove(file.Name())
	if err != nil {
		return err
	}

	utils.InfoLogger.Println("Extracted Steam CMD")

	return nil
}
