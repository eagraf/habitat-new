import { ThemedText } from "@/components/themed-text";
import { ThemedView } from "@/components/themed-view";
import { CameraCapturedPicture, CameraType, CameraView, useCameraPermissions } from "expo-camera";
import { Stack, useNavigation, useRouter } from "expo-router";
import { useRef, useEffect, useState } from "react";
import { Button, TouchableHighlight } from "react-native";

const Home = () => {
  const cameraRef = useRef<CameraView>(null);
  const router = useRouter();
  const [permission, requestPermission] = useCameraPermissions();
  const [facing, setFacing] = useState<CameraType>('back');

  async function uploadPhoto(photo: CameraCapturedPicture) {
    const res = await fetch(
      "https://privi.taile529e.ts.net/xrpc/network.habitat.uploadBlob",
      {
        method: 'POST',
        body: photo.base64,
        headers: {
          'Content-Type': photo.format
        },
      }
    )

    if (!res || !res.ok) {
      console.error("uploading photo blob", res)
      return
    }

    const upload = await res.json()
    const cid = upload["blob"]["cid"]["$link"]

    if (cid == "") {
      console.error("upload blob returned empty cid")
      return
    }

    const res2 = await fetch(
      "https://privi.taile529e.ts.net/xrpc/network.habitat.putRecord",
      {
        method: 'POST',
        body: JSON.stringify({
          collection: "network.habitat.photo",
          record: {
            ref: cid,
          },
          repo: "test"
        }),
      }
    )

    if (!res2 || !res2.ok) {
      console.error("uploading photo record", res2)
      return
    }

    console.log("uploaded photo record, response is: ", await res2.blob())
  }

  useEffect(() => {
    if (!permission?.granted) {
      requestPermission();
    }
  }, [permission]);

  if (!permission || !permission.granted) {
    // Camera permissions are still loading
    return <ThemedText>no permissions</ThemedText>;
  }

  return (
    <ThemedView style={{ flex: 1, alignItems: "center" }}>
      <Stack.Screen
        options={{
          title: "Camera",
          headerLeft: () => <TouchableHighlight onPress={() => router.navigate('/photos')}
          ><ThemedText>My Photos</ThemedText></TouchableHighlight>,
          headerRight: () =>
            <TouchableHighlight onPress={() => router.navigate('/signin')}
            ><ThemedText>Sign in</ThemedText></TouchableHighlight>
        }}
      />
      <CameraView style={{ flex: 1, width: '100%' }} facing={facing} ref={cameraRef} active={true} />

      <TouchableHighlight
        onPress={async () => {
          const photo = await cameraRef.current?.takePictureAsync({
            quality: 0.1,
          });
          if (!photo) {
            console.error("camera.takePictureAsync returned undefined")
          } else {
            uploadPhoto(photo)
          }
        }}
        style={{ padding: 8 }}
      >
        <ThemedText>Capture</ThemedText>
      </TouchableHighlight>

      <TouchableHighlight
        onPress={() => {
          setFacing(facing => (facing === 'back' ? 'front' : 'back'))
        }}
        style={{ padding: 8 }}
      >
        <ThemedText>Flip Camera</ThemedText>
      </TouchableHighlight>
    </ThemedView>
  );
};

export default Home;
