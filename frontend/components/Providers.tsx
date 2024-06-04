"use client"

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PropsWithChildren } from "react";
import { PreviousRouteProvider } from "./PreviousRouteContext";

const queryClient = new QueryClient()

export default function Providers({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <PreviousRouteProvider>
        {children}
      </PreviousRouteProvider>
    </QueryClientProvider>
  );
}
