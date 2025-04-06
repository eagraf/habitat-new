package types

import (
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// Types that need to be shared by many modules in internal/

type HabitatHostname string
type HabitatResolver func(syntax.DID) HabitatHostname
