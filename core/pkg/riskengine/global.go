package riskengine

import "sync"

// ReloadableEngine is implemented by Engine and YAMLEngine so risk rules can be reloaded from DB after CRUD.
type ReloadableEngine interface {
	ReloadFromDB() error
}

var (
	globalEngine ReloadableEngine
	globalMu     sync.Mutex
)

// RegisterEngine registers the risk engine used by the worker so API can trigger reload after rule create/update/delete.
func RegisterEngine(e ReloadableEngine) {
	globalMu.Lock()
	globalEngine = e
	globalMu.Unlock()
}

// ReloadGlobalFromDB reloads the registered engine's rules from DB. No-op if none registered.
func ReloadGlobalFromDB() error {
	globalMu.Lock()
	e := globalEngine
	globalMu.Unlock()
	if e == nil {
		return nil
	}
	return e.ReloadFromDB()
}
