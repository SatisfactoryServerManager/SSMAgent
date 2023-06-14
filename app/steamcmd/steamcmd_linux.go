package steamcmd

import (
	"archive/tar"
	"compress/gzip"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

func ExtractArchive(file *os.File) error {

	log.Println("Extracting Steam CMD...")
	gzipStream, err := os.Open(file.Name())
	if err != nil {
		return err
	}

	gzr, err := gzip.NewReader(gzipStream)
	if err != nil {
		return err
	}

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			break

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		if err == io.EOF {
			break
		}

		// the target location where the dir/file should be created
		target := filepath.Join(SteamDir, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			utils.CreateFolder(path.Dir(target))
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.ModePerm)
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}

	err = gzipStream.Close()
	utils.CheckError(err)

	err = gzr.Close()
	utils.CheckError(err)

	err = file.Close()
	utils.CheckError(err)

	err = os.Remove(file.Name())
	utils.CheckError(err)

	log.Println("Extracted Steam CMD")

	return nil
}
