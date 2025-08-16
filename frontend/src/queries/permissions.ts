import type { OAuthSession } from "@atproto/oauth-client-browser";
import { queryOptions } from "@tanstack/react-query";

export function listPermissions(session?: OAuthSession) {
  return queryOptions({
    queryKey: ["permissions"],
    queryFn: async () => {
      return { "com.habitat.test": [] }
      const response = await session?.fetchHandler(`/xrpc/com.habitat.listPermissions`, {
        headers: {
          'atproto-proxy': 'did:web:localhost-1.taile529e.ts.net#privi'
        }
      })
      const json: Record<string, string[]> = await response?.json();
      return json
    },
  })
}
