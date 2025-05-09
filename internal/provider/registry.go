package provider

import "fmt"

var registry = map[string]Provider{}

func Register(name string, p Provider) {
	registry[name] = p
}

func Get(name string) (Provider, error) {
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not registered", name)
	}
	return p, nil
}
