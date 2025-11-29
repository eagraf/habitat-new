import { ThemedText } from "@/components/themed-text";
import { ThemedView } from "@/components/themed-view";
import { useAuth } from "@/context/auth";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Image, Platform, useWindowDimensions } from "react-native";

const Tile = ({ cid, size }: { cid: string, size: number }) => {
    const { token } = useAuth()
    const src = `https://privi.dwelf-mirzam.ts.net/xrpc/network.habitat.getBlob?cid=${cid}&did=test`
    return <Image
        source={{
            uri: src,
            headers: {
                Authorization: `Bearer ${token}`,
                "Habitat-Auth-Method": "oauth",
            }
        }}
        style={{
            width: size,
            height: size,
        }}
        resizeMode="cover"
    />
}

const Photos = () => {
    const { fetchWithAuth } = useAuth()
    const { isLoading, data: photos, error } = useQuery({
        queryKey: ["photos"],
        queryFn: async () => {
            const res = await fetchWithAuth(
                `/xrpc/network.habitat.listRecords?collection=network.habitat.photo&repo=test` // TODO: repo
            )
            if (!res || !res.ok) {
                throw new Error("fetching photos: " + res.statusText + await res.text())
            }
            const list = await res.json()
            return list["records"] as { value: { ref: string } }[]
        }
    })
    const { width } = useWindowDimensions();
    // Determine tiles per row
    const tilesPerRow = Platform.OS === "web" ? 10 : 3;
    // Calculate tile width
    const tileSize = width / tilesPerRow;

    if (error) {
        return <ThemedText>{error.message}</ThemedText>
    }

    if (!photos || isLoading) {
        return <ThemedText>Loading ... </ThemedText>
    }

    return (
        <ThemedView style={{ flexDirection: "row", flexWrap: "wrap", }}>
            {photos.map(({ value }, i) => (
                <Tile key={i} cid={value.ref} size={tileSize} />
            ))}
        </ThemedView>
    );
}



export default Photos;
