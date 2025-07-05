package auth

import (
	"net/http"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

// DpopJWKBuilder is a function that builds a DPoP JWT for a given request type.
type DpopJWKBuilder func(req *http.Request, signer jose.Signer, session *dpopSession, nonce string) (string, error)

func getPDSJWKBuilder(htu string) DpopJWKBuilder {
	return func(req *http.Request, signer jose.Signer, session *dpopSession, nonce string) (string, error) {
		accessTokenHash := session.getAccessTokenHash()

		return jwt.Signed(signer).Claims(&dpopClaims{
			Claims: jwt.Claims{
				ID:       generateNonce(),
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
			Method:          req.Method,
			URL:             htu,
			Nonce:           nonce,
			AccessTokenHash: accessTokenHash,
		}).CompactSerialize()
	}
}

func getAuthServerJWKBuilder() DpopJWKBuilder {
	return func(req *http.Request, signer jose.Signer, session *dpopSession, nonce string) (string, error) {
		htu := req.URL.String()

		return jwt.Signed(signer).Claims(&dpopClaims{
			Claims: jwt.Claims{
				ID:       generateNonce(),
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
			Method: req.Method,
			URL:    htu,
			Nonce:  nonce,
		}).CompactSerialize()
	}
}
