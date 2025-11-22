import { ThemedText } from "@/components/themed-text";
import { useEffect } from "react";
import { Image } from "react-native";



const Photos = () => {
    useEffect(() => {
        // Returns []cid on which to call getBlob
        async function getAllPhotos(): Promise<string[]> {
            // TODO: take in habitat domain
            const res = await fetch(
                `https://privi.taile529e.ts.net/xrpc/network.habitat.listRecords?collection=network.habitat.photo&repo=test` // TODO: repo
            )
            if (!res || !res.ok) {
                throw new Error("failed to fetch photos")
            }

            const list = await res.json()
            const photos = list["records"]
            
            return photos.map((photo: any) => { return photo["ref"]})
        }
    }, [])
    return 
};

const Tile = ({ cid }: { cid: string }) => {
    return <Image src={`https://privi.taile529e.ts.net/xrpc/network.habitat.getBlob?cid=${cid}&did=test`} />
    
}

export default Photos;