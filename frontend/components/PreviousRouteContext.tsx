"use client"

import { usePathname } from "next/navigation"
import { PropsWithChildren, createContext, useContext, useEffect, useState } from "react"

const PreviousRouteGetterContext = createContext<string | undefined>(undefined)

export function usePreviousRoute() {
  return useContext(PreviousRouteGetterContext)
}

export function PreviousRouteProvider({ children }: PropsWithChildren) {
  const pathname = usePathname()
  const [currentRoute, setCurrentRoute] = useState<string | undefined>(undefined)
  const [prevRoute, setPreviousRoute] = useState<string | undefined>(undefined)
  useEffect(() => {
    setPreviousRoute(currentRoute)
    setCurrentRoute(pathname)
  }, [pathname])
  return <PreviousRouteGetterContext.Provider value={prevRoute}>
    {children}
  </PreviousRouteGetterContext.Provider>
}
