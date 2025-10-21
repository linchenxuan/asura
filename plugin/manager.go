package plugin

import (
	"fmt"
	"sync"
)

type pluginMgr struct {
	insMap map[string]map[string]map[string]Plugin // map{k : ft, v : map{k : fn, v : map{k : insname, v : ins}}}
}

var (
	_pluginMgr = &pluginMgr{
		insMap: make(map[string]map[string]map[string]Plugin),
	}
	_pluginLock = sync.RWMutex{}
)

func registerPluginIns(ft string, fn string, pn string, ins Plugin) error {
	_pluginLock.Lock()
	defer _pluginLock.Unlock()
	// factory type map
	m, ok := _pluginMgr.insMap[ft]
	if !ok {
		_pluginMgr.insMap[ft] = map[string]map[string]Plugin{ // key: factory type, value: <plugin name:<tag, ins>>
			fn: {
				pn: ins,
			},
		}
		return nil
	}
	// factory name map
	n, ok := m[fn]
	if !ok {
		m[fn] = map[string]Plugin{
			pn: ins,
		}
		return nil
	}
	// instance
	if _, ok := n[pn]; ok {
		return fmt.Errorf("plugin %s already exists", pn)
	}
	n[pn] = ins
	return nil
}
