package privi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"

	"github.com/eagraf/habitat-new/core/permissions"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/rs/zerolog/log"
)

type PutRecordRequest struct {
	Input   *agnostic.RepoPutRecord_Input
	Encrypt bool `json:"encrypt"`
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
func (s *Server) PutRecord(authInfo *xrpc.AuthInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PutRecordRequest
		slurp, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = json.Unmarshal(slurp, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Get the PDS endpoint for the caller's DID
		// If the caller does not have write access, the write will fail (assume privi is read-only premissions for now)
		did := authInfo.Did
		atid, err := syntax.ParseAtIdentifier(did)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.dir.Lookup(r.Context(), *atid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		inner, ok := s.servedByMe(id.DID)
		if !ok {
			// TODO: write helpful message
			http.Error(w, fmt.Sprintf("%s: did %s", errWrongServer.Error(), id.DID.String()), http.StatusBadRequest)
			return
		}

		err = inner.putRecord(req.Input)
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
func (s *Server) GetRecord(authInfo *xrpc.AuthInfo) http.HandlerFunc {
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

		// Try handling both handles and dids
		atid, err := syntax.ParseAtIdentifier(repo)
		if err != nil {
			// TODO: write helpful message
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.dir.Lookup(r.Context(), *atid)
		if err != nil {
			// TODO: write helpful message
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		targetDID := id.DID
		inner, ok := s.servedByMe(targetDID)
		if !ok {
			// TODO: write helpful message
			http.Error(w, fmt.Sprintf("%s: did %s", errWrongServer.Error(), id.DID.String()), http.StatusBadRequest)
			return
		}

		out, err := inner.getRecord(collection, rkey, syntax.DID(authInfo.Did))

		if errors.Is(err, ErrUnauthorized) {
			http.Error(w, ErrUnauthorized.Error(), http.StatusForbidden)
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
func (s *Server) pdsAuthMiddleware(next func(authInfo *xrpc.AuthInfo) http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, err := getAuthInfo(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		next(auth).ServeHTTP(w, r)
	})
}

// HACK: trust did
func getAuthInfo(r *http.Request) (*xrpc.AuthInfo, error) {
	accessJwt := ""
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 {
		accessJwt = authHeader[7:]
	}
	did := ""
	for _, cookie := range r.Cookies() {
		if cookie.Name == "access_token" {
			accessJwt = cookie.Value
		} else if cookie.Name == "did" {
			did = cookie.Value
		}
	}
	return &xrpc.AuthInfo{
		AccessJwt: accessJwt,
		Did:       did,
	}, nil
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
