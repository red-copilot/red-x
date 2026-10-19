package poc

import (
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Plugin{}
)

func Register(p Plugin) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[p.Meta().Name] = p
}

func Registry() []Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Plugin, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta().Name < out[j].Meta().Name })
	return out
}

func Find(name string) Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

func Counts() map[string]int {
	out := map[string]int{"poc": 0, "sniffer": 0, "brute": 0}
	for _, p := range Registry() {
		out[string(p.Meta().Type)]++
	}
	return out
}
