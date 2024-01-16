package savedecoder

import (
	"encoding/binary"
	"os"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

type SaveDecoder struct {
	File        *os.File
	CurrentSeek int
}

func NewSaveDecoder(file *os.File) SaveDecoder {
	newDecoder := SaveDecoder{}
	newDecoder.CurrentSeek = 0
	newDecoder.File = file

	return newDecoder
}

func (decoder *SaveDecoder) Seek(n int) {
	decoder.CurrentSeek += n
}

func (decoder *SaveDecoder) Close() error {
	if err := decoder.File.Close(); err != nil {
		return err
	}
	return nil
}

func (decoder SaveDecoder) DebugSeek() {
	utils.DebugLogger.Printf("Current Seek: %d\r\n", decoder.CurrentSeek)
}

func (decoder *SaveDecoder) ReadInt() uint16 {
	decoder.File.Seek(int64(decoder.CurrentSeek), 0)
	var i uint16
	binary.Read(decoder.File, binary.LittleEndian, &i)
	decoder.Seek(4)
	return i
}

func (decoder *SaveDecoder) ReadString() (string, error) {
	decoder.File.Seek(int64(decoder.CurrentSeek), 0)

	strlen := decoder.ReadInt() - 1
	decoder.File.Seek(int64(decoder.CurrentSeek), 0)

	buf := make([]byte, strlen)

	_, err := decoder.File.Read(buf)
	if err != nil {
		return "", err
	}

	decoder.Seek(int(strlen) + 1)
	return string(buf), nil

}
