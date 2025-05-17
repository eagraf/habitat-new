import { BrowserOAuthClient } from '@atproto/oauth-client-browser'

export const oauthClient = new BrowserOAuthClient({
  handleResolver: 'https://bsky.social',
  clientMetadata: {
    client_id: 'http://localhost?scope=atproto%20transition%3Ageneric',
    redirect_uris: ['http://127.0.0.1:3001/'],
    scope: 'atproto transition:generic',
    token_endpoint_auth_method: 'none'
  },
  allowHttp: true
})
