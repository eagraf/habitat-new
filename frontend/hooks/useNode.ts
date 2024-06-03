import { useQuery } from "@tanstack/react-query";

export interface Node {
  database_id: string
  state: {
    app_installations: Record<string, {
      driver: string
      id: string
      name: string
      registry_app_id: string
      registry_tag: string
      registry_url_base: string
      state: string
      user_id: string
      version: string
    }>
    users: Record<number, {
      certificate: string
      id: string
      username: string
    }>
  }
}

async function fetchNode() {
  const resp = await fetch("/habitat/api/node")
  const json = await resp.json() as Node
  return json
}

export default function useNode<T = Node>(select?: (node: Node) => T) {
  return useQuery({
    queryKey: ["node"],
    queryFn: fetchNode,
    select
  })
}
