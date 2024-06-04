"use client"

import { usePreviousRoute } from "@/components/PreviousRouteContext";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { useRouter, useSelectedLayoutSegment, useSelectedLayoutSegments } from "next/navigation";
import { PropsWithChildren, useEffect, useState } from "react";

export default function Modal({ children }: PropsWithChildren) {
  const [open, setOpen] = useState(false)
  const modalSegment = useSelectedLayoutSegment('modal')
  const router = useRouter()
  const previousRoute = usePreviousRoute()
  const topSegments = useSelectedLayoutSegments()

  useEffect(() => {
    setOpen(!!modalSegment)
  }, [modalSegment])

  return <Dialog open={open} onOpenChange={(open) => {
    if (!open) {
      if (previousRoute) {
        router.back()
      } else {
        router.replace("/" + topSegments.slice(0, -1).join("/"))
      }
    }
  }}>
    <DialogContent>{children}</DialogContent>
  </Dialog>;
}
