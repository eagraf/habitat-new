package privi

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	atprotocrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/golang-jwt/jwt/v5"

	"github.com/eagraf/habitat-new/core/permissions"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/rs/zerolog/log"
)

type PutRecordRequest struct {
	Collection string         `json:"collection"`
	Repo       string         `json:"repo"`
	Rkey       string         `json:"rkey"`
	Record     map[string]any `json:"record"`
}

type Server struct {
	// TODO: allow privy server to serve many stores, not just one user
	stores map[syntax.DID]*store
	// Used for resolving handles -> did, did -> PDS
	dir identity.Directory
}

func defaultEncrypter() Encrypter {
	return &NoopEncrypter{}
}

// NewServer returns a privi server.
func NewServer(didToStores map[syntax.DID]permissions.Store) *Server {
	server := &Server{
		stores: make(map[syntax.DID]*store),
		dir:    identity.DefaultDirectory(),
	}
	for did, perms := range didToStores {
		err := server.Register(did, perms)
		if err != nil {
			log.Err(err)
		}
	}
	return server
}

func (s *Server) Register(did syntax.DID, perms permissions.Store) error {
	_, ok := s.stores[did]
	if ok {
		return fmt.Errorf("existing privi store for this did: %s", did.String())
	}

	s.stores[did] = newStore(did, perms)
	return nil
}

// PutRecord puts a potentially encrypted record (see s.inner.putRecord)
func (s *Server) PutRecord(syntax.DID) http.HandlerFunc {
	fmt.Println("PriviServer PutRecord")
	return func(w http.ResponseWriter, r *http.Request) {
		var req PutRecordRequest
		slurp, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("here", string(slurp))
		err = json.Unmarshal(slurp, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println("here1")

		// Get the PDS endpoint for the caller's DID
		// If the caller does not have write access, the write will fail (assume privi is read-only premissions for now)

		did := req.Repo
		atid, err := syntax.ParseAtIdentifier(did)
		fmt.Println("parseatidentifier", atid, err)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		id, err := s.dir.Lookup(r.Context(), *atid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("Got PutRecord request", did, req)

		inner, ok := s.servedByMe(id.DID)
		if !ok {
			// TODO: write helpful message
			http.Error(w, fmt.Sprintf("%s: did %s", errWrongServer.Error(), id.DID.String()), http.StatusBadRequest)
			return
		}

		v := true
		err = inner.putRecord(req.Collection, req.Record, req.Rkey, &v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := w.Write([]byte("OK")); err != nil {
			log.Err(err).Msgf("error sending response for PutRecord request")
		}
	}
}

func (s *Server) servedByMe(did syntax.DID) (*store, bool) {
	store, ok := s.stores[did]
	return store, ok
}

var (
	errWrongServer = fmt.Errorf("did is not served by this privi instance:")
)

// Find desired did
// if other did, forward request there
// if our own did,
// --> if authInfo matches then fulfill the request
// --> otherwise verify requester's token via bff auth --> if they have permissions via permission store --> fulfill request

// GetRecord gets a potentially encrypted record (see s.inner.getRecord)
func (s *Server) GetRecord(callerDID syntax.DID) http.HandlerFunc {
	fmt.Println("PriviServer GetRecord")
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// cid := u.Query().Get("cid") -- TODO: enable get by this
		collection := u.Query().Get("collection")
		repo := u.Query().Get("repo")
		rkey := u.Query().Get("rkey")

		fmt.Println("here")

		// Try handling both handles and dids
		atid, err := syntax.ParseAtIdentifier(repo)
		if err != nil {
			// TODO: write helpful message
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("here1")

		id, err := s.dir.Lookup(r.Context(), *atid)
		if err != nil {
			// TODO: write helpful message
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("here2")

		targetDID := id.DID
		inner, ok := s.servedByMe(targetDID)
		if !ok {
			// TODO: write helpful message
			http.Error(w, fmt.Sprintf("%s: did %s", errWrongServer.Error(), id.DID.String()), http.StatusBadRequest)
			return
		}
		fmt.Println("here3")

		out, err := inner.getRecord(collection, rkey, callerDID)

		fmt.Println("out, err", string(out), err)

		if errors.Is(err, ErrUnauthorized) {
			http.Error(w, ErrUnauthorized.Error(), http.StatusForbidden)
			return
		} else if errors.Is(err, ErrRecordNotFound) {
			http.Error(w, ErrRecordNotFound.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := w.Write(out); err != nil {
			log.Err(err).Msgf("error sending response for GetRecord request")
		}
	}
}

// This creates the xrpc.Client to use in the inner privi requests
// TODO: this should actually pull out the requested did from the url param or input and re-direct there. (Potentially move this lower into the fns themselves).
// This would allow for requesting to any pds through these routes, not just the one tied to this habitat node.
func (s *Server) pdsAuthMiddleware(next func(syntax.DID) http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		did, err := s.getCaller(r)
		fmt.Println("getCaller err", err)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		next(did).ServeHTTP(w, r)
	})
}

type serviceJwtHeader struct {
	Alg string `json:"alg"`
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
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
	Lxm string `json:"lxm"`
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

func (s *Server) validateBearerToken(ctx context.Context, token string) (string, error) {
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

	if time.Now().Unix() > payload.Exp {
		return "", errors.New("token expired")
	}

	// TODO: we should probably vaildate that this is the intended did but it
	// technically doesn't matter for now
	// if ownDid != payload.aud {
	// 	return "", errors.New("invalid audience")
	// }

	if payload.Lxm != "com.habitat.getRecord" {
		return "", errors.New("unsupported lexicon method")
	}

	msg := []byte(strings.Join(parts[0:2], "."))
	sig, err := base64.URLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.Join(errors.New("failed to decode signature"), err)
	}

	id, err := s.dir.LookupDID(ctx, syntax.DID(payload.Iss))
	if err != nil {
		return "", errors.Join(errors.New("failed to lookup identity"), err)
	}
	publicKey, err := id.PublicKey()
	if err != nil {
		return "", errors.Join(errors.New("failed to get public key"), err)
	}

	if header.Alg == "ES256K" {
		if _, ok := publicKey.(*atprotocrypto.PublicKeyK256); !ok {
			return "", errors.New("invalid key type")
		}
	} else if header.Alg == "ES256" {
		if _, ok := publicKey.(*atprotocrypto.PublicKeyP256); !ok {
			return "", errors.New("invalid key type")
		}
	} else {
		return "", errors.New("unsupported algorithm")
	}

	err = publicKey.HashAndVerify(msg, sig)
	if err != nil {
		return "", errors.Join(errors.New("failed to verify signature"), err)
	}

	return payload.Iss, nil
}

// HACK: trust did
func (s *Server) getCaller(r *http.Request) (syntax.DID, error) {
	fmt.Println(r.Header)
	authHeader := r.Header.Get("Authorization")
	token := strings.Split(authHeader, "Bearer ")[1]
	fmt.Println("token from header", token)
	jwt.RegisterSigningMethod("ES256K", func() jwt.SigningMethod {
		return &SigningMethodSecp256k1{
			alg:      "ES256K",
			hash:     crypto.SHA256,
			toOutSig: toES256K, // R || S
			sigLen:   64,
		}
	})
	jwtToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		did, err := t.Claims.GetIssuer()
		if err != nil {
			return nil, err
		}
		id, err := s.dir.LookupDID(r.Context(), syntax.DID(did))
		if err != nil {
			return "", errors.Join(errors.New("failed to lookup identity"), err)
		}
		return id.PublicKey()
	}, jwt.WithValidMethods([]string{"ES256K"}), jwt.WithoutClaimsValidation())

	if err != nil {
		return "", err
	}
	if jwtToken == nil {
		return "", fmt.Errorf("jwtToken is nil")
	}
	did, err := jwtToken.Claims.GetIssuer()
	if err != nil {
		return "", err
	}
	fmt.Println("issuer, err", did, err)
	return syntax.DID(did), err
}

func (s *Server) GetRoutes() []api.Route {
	return []api.Route{
		api.NewBasicRoute(
			http.MethodPost,
			"/xrpc/com.habitat.putRecord",
			s.pdsAuthMiddleware(s.PutRecord),
		),
		api.NewBasicRoute(
			http.MethodGet,
			"/xrpc/com.habitat.getRecord",
			s.pdsAuthMiddleware(s.GetRecord),
		),
	}
}
