import { ThemedText } from "@/components/themed-text";
import { ThemedView } from "@/components/themed-view";
import { useEffect, useState } from "react";
import { Image, Platform, useWindowDimensions } from "react-native";

const Tile = ({ cid, size }: { cid: string, size: number }) => {
    const src = `https://privi.taile529e.ts.net/xrpc/network.habitat.getBlob?cid=${cid}&did=test`
    return <Image
        source={{
            uri: src,
        }}
        style={{
            width: size,
            height: size,
        }}
        resizeMode="cover"
    />
}

const Photos = () => {
    const [photos, setPhotos] = useState<string[]>([]);
    const [loading, setLoading] = useState<boolean>(true);

    const { width } = useWindowDimensions();
    // Determine tiles per row
    const tilesPerRow = Platform.OS === "web" ? 10 : 3;
    // Calculate tile width
    const tileSize = width / tilesPerRow;

    useEffect(() => {
        // Returns []cid on which to call getBlob
        const fetchPhotos = async () => {
            // TODO: take in habitat domain
            const res = await fetch(
                `https://privi.taile529e.ts.net/xrpc/network.habitat.listRecords?collection=network.habitat.photo&repo=test` // TODO: repo
            )
            if (!res || !res.ok) {
                throw new Error("failed to fetch photos")
            }

            const list = await res.json()
            const photos = list["records"]

            setPhotos(photos.map((photo: any) => { return photo["value"]["ref"] }))
            setLoading(false)
        }

        fetchPhotos();
    }, [])

    if (loading) {
        return <ThemedText>Loading ... </ThemedText>
    }

    return (
        <ThemedView
            style={{
                flexDirection: "row",
                flexWrap: "wrap",
            }}
        >
            {photos.map((cid) => (
                <Tile
                    key={cid}
                    cid={cid}
                    size={tileSize}
                />
            ))}
        </ThemedView>
    );
}



export default Photos;