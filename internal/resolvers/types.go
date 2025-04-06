package resolvers

import "github.com/bluesky-social/indigo/atproto/crypto"

type HabitatHostResolver func(did string) (habitatHost string, err error)

type PublicKeyResolver func(did string) (publicKey crypto.PublicKey, err error)

type PrivateKeyResolver func(did string) (privateKey crypto.PrivateKey, err error)

// TODO resolve the DID from a host
