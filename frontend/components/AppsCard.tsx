"use client"
import useNode, { Node } from "@/hooks/useNode"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Card, CardHeader, CardTitle } from "@/components/ui/card"

function selectApps(node: Node) {
  return Object.values(node.state.app_installations)
}

interface AppsCardProps {
  className?: string
}

export default function AppsCard({ className }: AppsCardProps) {
  const apps = useNode(selectApps)
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>Installed Apps</CardTitle>
      </CardHeader>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>App</TableHead>
            <TableHead>Version</TableHead>
          </TableRow>
        </TableHeader>
        {apps.isLoading ? (
          <TableBody>
            <tr>
              <td colSpan={2}>Loading...</td>
            </tr>
          </TableBody>
        ) : (
          <TableBody>
            {apps.data?.map((app) => (
              <TableRow key={app.id}>
                <TableCell>{app.name}</TableCell>
                <TableCell>{app.version}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        )}
      </Table>
    </Card>
  )
}
