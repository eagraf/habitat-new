"use client"

import useNode, { Node } from "@/hooks/useNode";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Card, CardHeader, CardTitle } from "@/components/ui/card"

const selectUsers = (node: Node) => {
  return Object.values(node.state.users)
};

interface UsersCardProps {
  className?: string
}


export default function UsersCard({ className }: UsersCardProps) {
  const users = useNode(selectUsers)
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>Users</CardTitle>
      </CardHeader>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Usernames</TableHead>
          </TableRow>
        </TableHeader>
        {users.isLoading ? (
          <TableBody>
            <TableRow>
              <td colSpan={2}>Loading...</td>
            </TableRow>
          </TableBody>
        ) : (
          <TableBody>
            {users.data?.map((user) => (
              <TableRow key={user.id}>
                <TableCell>{user.username}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        )}
      </Table>
    </Card>
  )
}
