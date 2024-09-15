package deploy

import "github.com/goccy/go-yaml"

type ConfigFile struct {
	file []byte
	env  string
}

func ProdConfig(file []byte) ConfigFile {
	return ConfigFile{file, ProdENV}
}

func StagingConfig(file []byte) ConfigFile {
	return ConfigFile{file, StagingEnv}
}

func DevConfig(file []byte) ConfigFile {
	return ConfigFile{file, DevENV}
}
func GlobalConfig(file []byte) ConfigFile {
	return ConfigFile{file, ""}
}

func LoadConfig[Cfg any](files ...ConfigFile) *Cfg {
	var out Cfg

	for _, file := range files {
		if file.env == ENV || file.env == "" {
			if err := yaml.Unmarshal(file.file, &out); err != nil {
				panic(err)
			}
		}
	}

	return &out
}
