import { ThemedText } from "@/components/themed-text";
import { ThemedView } from "@/components/themed-view";
import { useState } from "react";
import { Button, TextInput, TouchableHighlight } from "react-native";
import { exchangeCodeAsync, makeRedirectUri, useAuthRequest } from 'expo-auth-session';

const SignIn = () => {
  const [handle, setHandle] = useState<string>("sashankg.bsky.social");
  const [request, response, promptAsync] = useAuthRequest(
    {
      extraParams: {
        handle,
      },
      clientId: "https://sashankg.github.io/client-metadata.json",
      scopes: [],
      redirectUri: makeRedirectUri({
        scheme: 'habitat.camera',
      }),
    },
    {
      authorizationEndpoint:
        "https://habitat-new.onrender.com/oauth/authorize",
      tokenEndpoint: "https://habitat-new.onrender.com/oauth/token",

    }
  );

  return (
    <ThemedView style={{ flex: 1 }}>
      <TextInput onChangeText={setHandle} value={handle} style={{ backgroundColor: 'gray' }}/>
        <Button
          onPress={async () => {
          const type = await promptAsync();
          console.log(type)
        }}
        title="Sign in" />
    </ThemedView>
  );
};

export default SignIn;
