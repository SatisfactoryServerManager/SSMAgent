//go:build linux
// +build linux

package savemanager

func GetSaveDir() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	saveDir, err := filepath.Abs(path.Join(homedir, ".config", "Epic", "FactoryGame", "Saved", "SaveGames", "server"))
	if err != nil {
		return "", err
	}

	return saveDir, nil
}
