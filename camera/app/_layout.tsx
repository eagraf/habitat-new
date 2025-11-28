import {
  DarkTheme,
  DefaultTheme,
  ThemeProvider,
} from "@react-navigation/native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { useColorScheme } from "@/hooks/use-color-scheme";
import { ThemedView } from "@/components/themed-view";
import { useState } from "react";
import SignIn from "./signin";
import AuthContext from "./AuthContext";

export const unstable_settings = {
  anchor: "(tabs)",
};

export default function RootLayout() {
  const colorScheme = useColorScheme();
  const { bottom } = useSafeAreaInsets();

  const [auth, setAuth] = useState(null);

  // Pass auth and setAuth so signin can update it
  /*
  return (
    <AuthContext.Provider value={{ auth, setAuth }}>
      {!auth ? <SignIn setAuth={setAuth} /> : 
      <ThemeProvider value={colorScheme === "dark" ? DarkTheme : DefaultTheme}>
      <ThemedView style={{ flex: 1, paddingBottom: bottom }}>
        <Stack />
      </ThemedView>
      <StatusBar style="auto" />
    </ThemeProvider>}
    </AuthContext.Provider>
  );
   */

  return (
    <ThemeProvider value={colorScheme === "dark" ? DarkTheme : DefaultTheme}>
      <ThemedView style={{ flex: 1, paddingBottom: bottom }}>
        <Stack />
      </ThemedView>
      <StatusBar style="auto" />
    </ThemeProvider>
  );
}
