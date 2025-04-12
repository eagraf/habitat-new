package bffauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
)

type serviceAuthClient struct {
	localPDSHost string
}

func NewServiceAuthClient(localPDSHost string) Client {
	return &serviceAuthClient{
		localPDSHost,
	}
}

// GetToken implements Client.
func (s *serviceAuthClient) GetToken(did string) (string, error) {
	client := &xrpc.Client{
		Host: s.localPDSHost,
	}

	ctx := context.Background()

	resp, err := atproto.ServerGetServiceAuth(ctx, client, did, 0, "com.habitat.getRecord")
	if err != nil {
		return "", err
	}

	return resp.Token, nil
}

type serviceAuthServer struct {
	dir identity.Directory
}

func NewServiceAuthServer(dir identity.Directory) Server {
	return &serviceAuthServer{
		dir,
	}
}

// ValidateToken implements Server.
// spec: https://atproto.com/specs/xrpc#service-proxying
// reference impl: https://github.com/bluesky-social/atproto/blob/f329c56454a178ef0d31671224f7370abe05142b/packages/xrpc-server/src/auth.ts#L72
func (s *serviceAuthServer) ValidateToken(token string) (string, error) {
	ctx := context.Background()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("poorly formatted jwt")
	}

	header, err := parseHeader(parts[0]) // header
	if err != nil {
		return "", errors.Join(fmt.Errorf("failed to parse header"), err)
	}

	payload, err := parsePayload(parts[1])
	if err != nil {
		return "", errors.Join(fmt.Errorf("failed to parse payload"), err)
	}

	if time.Now().Unix() > payload.exp {
		return "", errors.New("token expired")
	}

	// TODO: we should probably vaildate that this is the intended did but it
	// technically doesn't matter for now
	// if ownDid != payload.aud {
	// 	return "", errors.New("invalid audience")
	// }

	if payload.lxm != "com.habitat.getRecord" {
		return "", errors.New("unsupported lexicon method")
	}

	msg := []byte(strings.Join(parts[0:2], "."))
	sig, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.Join(errors.New("failed to decode signature"), err)
	}

	id, err := s.dir.LookupDID(ctx, syntax.DID(payload.iss))
	publicKey, err := id.PublicKey()
	if err != nil {
		return "", errors.Join(errors.New("failed to get public key"), err)
	}

	if header.alg == "ES256K" {
		if _, ok := publicKey.(*crypto.PublicKeyK256); !ok {
			return "", errors.New("invalid key type")
		}
	} else if header.alg == "ES256" {
		if _, ok := publicKey.(*crypto.PublicKeyP256); !ok {
			return "", errors.New("invalid key type")
		}
	} else {
		return "", errors.New("unsupported algorithm")
	}

	err = publicKey.HashAndVerify(msg, sig)
	if err != nil {
		return "", errors.Join(errors.New("failed to verify signature"), err)
	}

	return payload.iss, nil
}

type serviceJwtHeader struct {
	alg string
}

func parseHeader(header string) (*serviceJwtHeader, error) {
	bytes, err := base64.URLEncoding.DecodeString(header)
	if err != nil {
		return nil, err
	}
	var v serviceJwtHeader
	err = json.Unmarshal(bytes, &v)
	return &v, err
}

type serviceJwtPayload struct {
	iss string
	aud string
	exp int64
	lxm string
	jti string
}

func parsePayload(payload string) (*serviceJwtPayload, error) {
	bytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	var v serviceJwtPayload
	err = json.Unmarshal(bytes, &v)
	return &v, err
}
