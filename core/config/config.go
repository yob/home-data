package config

import (
	"fmt"

	"github.com/pelletier/go-toml"
	"github.com/pelletier/go-toml/query"
)

type ConfigFile struct {
	tree *toml.Tree
}

type ConfigSection struct {
	tree *toml.Tree
}

func NewConfigFromFile(path string) (*ConfigFile, error) {
	tree, err := toml.LoadFile("config.toml")
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
	q, _ := query.Compile("$..[adapter]")
	// TODO return empty slice on error
	results := q.Execute(file.tree)
	for ii, item := range results.Values() {
		fmt.Printf("Query result %d: %v\n", ii, item)
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
