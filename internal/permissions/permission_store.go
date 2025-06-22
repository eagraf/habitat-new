package permissions

import (
	_ "embed"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

type Store interface {
	HasPermission(
		didstr string,
		nsid string,
		rkey string,
		write bool,
	) (bool, error)
}

type store struct {
	enforcer *casbin.Enforcer
}

//go:embed model.conf
var modelStr string

func NewStore(adapter persist.Adapter) (Store, error) {
	m, err := model.NewModelFromString(modelStr)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}
	return &store{
		enforcer: enforcer,
	}, nil
}

// HasPermission implements PermissionStore.
func (p *store) HasPermission(
	didstr string,
	nsid string,
	rkey string,
	write bool,
) (bool, error) {
	act := "read"
	if write {
		act = "write"
	}
	return p.enforcer.Enforce(didstr, getObject(nsid, rkey), act)
}

func getObject(nsid string, rkey string) string {
	return nsid + "." + rkey
}
