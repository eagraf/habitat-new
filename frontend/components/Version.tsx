"use client"

import { useQuery } from "@tanstack/react-query"

const fetchVersion = async () => {
  const response = await fetch("/habitat/api/version");
  const data = await response.text();
  return data;
}

export default function Version() {
  const version = useQuery({ queryKey: ["version"], queryFn: fetchVersion })
  if (version.isLoading || true) {
  }
  return version.data
}
