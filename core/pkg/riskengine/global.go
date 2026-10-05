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

// RegisterEngine registers the long-lived runtime rescore engine so rule create/update/delete reloads it.
// Other evaluators build a fresh engine per run and need no reload.
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
