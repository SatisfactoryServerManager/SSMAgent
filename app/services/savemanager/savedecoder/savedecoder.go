package savedecoder

import (
	"encoding/binary"
	"fmt"
	"os"
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
	fmt.Printf("Current Seek: %d\r\n", _currentSeek)
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

	Seek(int(strlen))
	return string(buf), nil

}
