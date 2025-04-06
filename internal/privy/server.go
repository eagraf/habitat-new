package privy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/api/agnostic"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"

	"github.com/eagraf/habitat-new/internal/bffauth"
	"github.com/eagraf/habitat-new/internal/node/api"
	"github.com/rs/zerolog/log"
)

type PutRecordRequest struct {
	Input   *agnostic.RepoPutRecord_Input
	Encrypt bool `json:"encrypt"`
}

type Server struct {
	inner *store
	// Used to figure out where to route requests given a DID
	habitatResolver func(string) string
	// Used for resolving handles -> did, did -> PDS
	dir identity.Directory
	// The local pds host this server is tied to
	localPDSHost string
	//
	bffClient bffauth.Client
}

// makeXrpcClient returns an xrpc.Client that can make requests to the PDS at the given did
// It takes in authInfo because that is
func (s *Server) makeXrpcClient(authInfo *xrpc.AuthInfo, did string) *xrpc.Client {
	// If th
	if authInfo.Did == did {
		return &xrpc.Client{
			Auth: authInfo,
		}
	}

	return nil
}

func NewServer(localPDSHost string, habitatResolver func(string) string, enc Encrypter) *Server {
	return &Server{
		pdsHost: pdsHost,
		inner: &store{
			e: enc,
		},
		habitatResolver: habitatResolver,
		localPDSHost:    localPDSHost,
	}
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

		xrpcClient := &xrpc.Client{
			Host: s.localPDSHost,
			Auth: authInfo,
		}
		out, err := s.inner.putRecord(r.Context(), xrpcClient, req.Input, req.Encrypt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		slurp, err = json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := w.Write(slurp); err != nil {
			log.Err(err).Msgf("error sending response for PutRecord request")
		}
	}
}

func getRecordParamsFromURL(u *url.URL) (cid, collection, repo, rkey string, err error) {
	params, err := url.Parse(u.String())
	if err != nil {
		return "", "", "", "", nil
	}
	cid = params.Query().Get("cid")
	collection = params.Query().Get("collection")
	repo = params.Query().Get("repo")
	rkey = params.Query().Get("rkey")
	return
}

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

		cid := u.Query().Get("cid")
		collection := u.Query().Get("collection")
		repo := u.Query().Get("repo")
		rkey := u.Query().Get("rkey")

		// Try handling both handles and dids
		id, err := s.dir.LookupHandle(r.Context(), syntax.Handle(repo))
		if err != nil {
			id, err = s.dir.LookupDID(r.Context(), syntax.DID(repo))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		cli := &xrpc.Client{
			Auth: authInfo,
			Host: s.localPDSHost,
		}

		var out *agnostic.RepoGetRecord_Output
		// If trying to get data from a PDS not managed by habitat
		if id.PDSEndpoint() != s.localPDSHost {
			// get bff token
			token, err := s.bffClient.GetToken(string(id.DID))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Wack -- we're overloading AccessJwt to also pass around habitat managed tokens
			// Do this for ease for now so i can re-use xrpc client with PDS notions of auth but with Habitat notions of auth
			cli.Auth = &xrpc.AuthInfo{
				AccessJwt: token,
			}
			// set header
			cli.Host = s.habitatResolver(string(id.DID))
			out, err = agnostic.RepoGetRecord(r.Context(), cli, cid, collection, string(id.DID), rkey)
		} else {
			// Local: call inner.getRecord
			out, err = s.inner.getRecord(r.Context(), cli, cid, collection, string(id.DID), rkey)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		slurp, err := json.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(slurp); err != nil {
			log.Err(err).Msgf("error sending response for GetRecord request")
		}
	}
}

// This creates the xrpc.Client to use in the inner privy requests
// TODO: this should actually pull out the requested did from the url param or input and re-direct there. (Potentially move this lower into the fns themselves).
// This would allow for requesting to any pds through these routes, not just the one tied to this habitat node.
func (s *Server) pdsAuthMiddleware(next func(authInfo *xrpc.AuthInfo) http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessJwt, err := getAccessJwt(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		bearer := authHeader[7:]
		auth := &xrpc.AuthInfo{
			AccessJwt: bearer,
		}
		next(auth).ServeHTTP(w, r)
	})
}

func getAccessJwt(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 {
		return authHeader[7:], nil
	}
	for _, cookie := range r.Cookies() {
		if cookie.Name == "access_token" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("missing auth info")
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
