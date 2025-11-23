import { ThemedText } from "@/components/themed-text";
import { ThemedView } from "@/components/themed-view";
import { useEffect, useState } from "react";
import { Button, TextInput, TouchableHighlight } from "react-native";
import * as AuthSession from 'expo-auth-session';
import { makeRedirectUri, useAuthRequest } from 'expo-auth-session';

const SignIn = ({ setAuth }: { setAuth: React.Dispatch<React.SetStateAction<any>>; }) => {
  const [handle, setHandle] = useState<string>("sashankg.bsky.social");
  const clientId = "https://sashankg.github.io/client-metadata.json" // fake for now

  const domain = "privi.taile529e.ts.net" // habitat-new.onrender.com
  const discoveryDoc = {
    authorizationEndpoint:
      `https://${domain}/oauth/authorize`,
    tokenEndpoint: `https://${domain}/oauth/token`,
  }

  const redirectUri = makeRedirectUri({
        scheme: 'habitat.camera',
      })
  console.log("redirect uri", redirectUri)

  const [request, response, promptAsync] = useAuthRequest(
    {
      extraParams: {
        handle,
      },
      clientId: clientId,
      scopes: [],
      redirectUri: redirectUri,
    },
    discoveryDoc,
  );

  useEffect(() => {
    console.log("response", response)
    if (response?.type === 'success') {
      const { code } = response.params;
      // Now, exchange this code for an access token
      exchangeCodeForToken(code);
    }
  }, [response]);

  const exchangeCodeForToken = async (code: string) => {
    try {
      const tokenResponse = await AuthSession.exchangeCodeAsync(
        {
          clientId: clientId,
          code: code,
          redirectUri: AuthSession.makeRedirectUri({ scheme: 'habitat.camera' }),
          extraParams: {
            // Include client_secret if required by your provider and handled securely
          },
        },
        discoveryDoc,
      );
      // tokenResponse.accessToken will contain your access token
      // tokenResponse.refreshToken may contain a refresh token
      console.log('Access Token:', tokenResponse.accessToken);
      // Store and use the access token
    } catch (error) {
      console.error('Error exchanging code for token:', error);
    }
  };

  return (
    <ThemedView style={{ flex: 1, justifyContent: "center", alignItems: "center" }}>
      <TextInput onChangeText={setHandle} value={handle} style={{ backgroundColor: 'gray' }} />
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
