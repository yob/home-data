package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"
	"github.com/pelletier/go-toml/query"
)

type ConfigFile struct {
	tree *toml.Tree
}

type ConfigSection struct {
	tree *toml.Tree
}

func FindConfigPath() (string, error) {
	binPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	binDir, err := filepath.Abs(filepath.Dir(binPath))
	if err != nil {
		return "", err
	}

	nextToBin := binDir + "/config.toml"
	if _, err := os.Stat(nextToBin); err == nil {
		return nextToBin, nil
	}

	etcConfig := "/etc/home-data/config.toml"
	if _, err := os.Stat(etcConfig); err == nil {
		return etcConfig, nil
	}

	return "", fmt.Errorf("Unable to find config file. Checked '%s' and '%s'", nextToBin, etcConfig)
}

func NewConfigFromFile(path string) (*ConfigFile, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}
	return &ConfigFile{
		tree: tree,
	}, nil
}

func (file *ConfigFile) Section(name string) (*ConfigSection, error) {
	res := file.tree.Get(name)
	subTree, ok := res.(*toml.Tree)
	if !ok {
		return nil, fmt.Errorf("section '%s' not found", name)
	}
	return &ConfigSection{
		tree: subTree,
	}, nil
}

func (file *ConfigFile) AdapterSections() []*ConfigSection {
	sections := make([]*ConfigSection, 0)
	query, err := query.Compile("$..[?(adapterOnly)]")
	if err != nil {
		// TODO do something better than just printing out
		fmt.Printf("err: %v", err)
		return sections
	}

	query.SetFilter("adapterOnly", func(node interface{}) bool {
		if tree, ok := node.(*toml.Tree); ok {
			return tree.Has("adapter")
		}
		return false
	})

	results := query.Execute(file.tree)
	for _, item := range results.Values() {
		itemTree, ok := item.(*toml.Tree)
		if ok {
			sections = append(sections, &ConfigSection{tree: itemTree})
		}
	}
	return sections
}

func (section *ConfigSection) GetString(key string) (string, error) {
	value := section.tree.Get(key)
	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("key '%s' is not a string", key)
	}
	return strValue, nil
}

func (section *ConfigSection) GetStringMap(key string) (map[string]string, error) {
	value := section.tree.Get(key)
	if value == nil {
		return make(map[string]string), fmt.Errorf("key '%s' not found", key)
	}
	valueTree, ok := value.(*toml.Tree)
	if !ok {
		return make(map[string]string), fmt.Errorf("key '%s' is not a tree", key)
	}
	result := make(map[string]string)
	for k, v := range valueTree.ToMap() {
		strValue, ok := v.(string)
		if ok {
			result[k] = strValue
		}
	}
	return result, nil
}

func (section *ConfigSection) GetStringSlice(key string) ([]string, error) {
	value := section.tree.GetArray(key)
	strValue, ok := value.([]string)
	if !ok {
		return []string{}, fmt.Errorf("key '%s' is not an array of strings", key)
	}
	return strValue, nil
}

func (section *ConfigSection) GetInt64(key string) (int64, error) {
	value := section.tree.Get(key)
	intValue, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("key '%s' is not an int64", key)
	}
	return intValue, nil
}

func (section *ConfigSection) String() string {
	return section.tree.String()
}
