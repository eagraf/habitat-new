package permissions

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bradenaw/juniper/xmaps"
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
	RemoveLexiconReadPermission(
		didstr string,
		nsid string,
	) error
	ListReadPermissionsByLexicon() (map[string][]string, error)
}

type store struct {
	enforcer *casbin.Enforcer
	adapter  persist.Adapter
}

//go:embed model.conf
var modelStr string

func NewStore(adapter persist.Adapter, autoSave bool) (Store, error) {
	m, err := model.NewModelFromString(modelStr)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}
	// Auto-Save allows for single policy updates to take effect dynamically.
	// https://casbin.org/docs/adapters/#autosave
	enforcer.EnableAutoSave(autoSave)
	return &store{
		enforcer: enforcer,
		adapter:  adapter,
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

// TODO: do some validation on input
func (p *store) AddLexiconReadPermission(
	didstr string,
	nsid string,
) error {
	fmt.Println("Got add", didstr, nsid)
	_, err := p.enforcer.AddPolicy(didstr, getObject(nsid, "*"), Read.String(), "allow")
	if err != nil {
		return err
	}
	return p.adapter.SavePolicy(p.enforcer.GetModel())
}

// TODO: do some validation on input
func (p *store) RemoveLexiconReadPermission(
	didstr string,
	nsid string,
) error {
	// TODO: should we actually be adding a deny here instead of just removing allow?
	_, err := p.enforcer.RemovePolicy(didstr, getObject(nsid, "*"), Read.String(), "allow")
	if err != nil {
		return err
	}
	return p.adapter.SavePolicy(p.enforcer.GetModel())
}

func (p *store) ListReadPermissionsByLexicon() (map[string][]string, error) {
	objs, err := p.enforcer.GetAllObjects()
	if err != nil {
		fmt.Println("enforcer error", err.Error())
		return nil, err
	}

	res := make(map[string][]string)
	for _, obj := range objs {
		perms, err := p.enforcer.GetImplicitUsersForResource(obj)
		if err != nil {
			fmt.Println("enforcer error get users", err.Error())
			return nil, err
		}
		users := make(xmaps.Set[string], 0)
		for _, perm := range perms {
			// Format of perms is [[bob data2 write] [alice data2 read] [alice data2 write]]
			if perm[2] == Read.String() {
				users.Add(perm[0])
			}
		}
		res[strings.TrimSuffix(obj, ".*")] = slices.Collect(maps.Keys(users))
	}

	fmt.Println("passed")
	return res, nil
}

func getObject(nsid string, rkey string) string {
	return nsid + "." + rkey
}

// List all permissions (lexicon -> [](users | groups))
// Add a permission on a lexicon for a user or group
// Remove a permission on a lexicon for a user or group
