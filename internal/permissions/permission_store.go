package permissions

import (
	_ "embed"
	"errors"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// enum defining the possible actions a user has permission to do on an object.
// an object is either a lexicon or a lexicon + record key
type Action int

const (
	Read Action = iota
	Write
)

var actionNames = map[Action]string{
	Read:  "read",
	Write: "write",
}

func (a Action) String() string {
	return actionNames[a]
}

type Store interface {
	HasPermission(
		didstr string,
		nsid string,
		rkey string,
		act Action,
	) (bool, error)
	AddLexiconReadPermission(
		didstr string,
		nsid string,
	) error
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
	act Action,
) (bool, error) {
	return p.enforcer.Enforce(didstr, getObject(nsid, rkey), act.String())
}

func (p *store) AddLexiconReadPermission(
	didstr string,
	nsid string,
) error {
	// TODO: probably wrong
	p.enforcer.AddRolesForUser(didstr, []string{"read"}, nsid)
	return errors.ErrUnsupported
}

func (p *store) RemoveLexiconReadPermission(
	didstr string,
	nsid string,
) error {
	// TODO: fill me in
	return errors.ErrUnsupported
}

func (p *store) ListPermissionsForLexicon(nsid string) ([]string, error) {
	return nil, errors.ErrUnsupported
}

func getObject(nsid string, rkey string) string {
	return nsid + "." + rkey
}

// List all permissions (lexicon -> [](users | groups))
// Add a permission on a lexicon for a user or group
// Remove a permission on a lexicon for a user or group
