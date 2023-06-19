package savedecoder

import (
	"encoding/binary"
	"os"

	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
)

var (
	_currentSeek = 0
	File         *os.File
)

func Seek(n int) {
	_currentSeek += n
}

func Reset() {
	if File != nil {
		File.Close()
		File = nil
	}
	_currentSeek = 0
}

func DebugSeek() {
	utils.DebugLogger.Printf("Current Seek: %d\r\n", _currentSeek)
}

func ReadInt() uint16 {
	File.Seek(int64(_currentSeek), 0)
	var i uint16
	binary.Read(File, binary.LittleEndian, &i)
	Seek(4)
	return i
}

func ReadString() (string, error) {
	File.Seek(int64(_currentSeek), 0)
	strlen := ReadInt() - 1
	File.Seek(int64(_currentSeek), 0)

	buf := make([]byte, strlen)

	_, err := File.Read(buf)
	if err != nil {
		return "", err
	}

	Seek(int(strlen) + 1)
	return string(buf), nil

}
