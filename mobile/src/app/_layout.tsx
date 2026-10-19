import { Inter_400Regular, Inter_500Medium, Inter_600SemiBold } from '@expo-google-fonts/inter';
import { PlusJakartaSans_700Bold, PlusJakartaSans_800ExtraBold } from '@expo-google-fonts/plus-jakarta-sans';
import { useFonts } from 'expo-font';
import { router, Stack, usePathname } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import * as WebBrowser from 'expo-web-browser';
import { useEffect } from 'react';
import { View } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { colors } from '../lib/theme';
import { SessionProvider, useSession } from '../lib/session';

// On web the bank's approval page opens in a popup; this closes it and hands the result back.
WebBrowser.maybeCompleteAuthSession();

const PUBLIC = ['/', '/login', '/connected'];

function Guard() {
  const { user } = useSession();
  const path = usePathname();
  useEffect(() => {
    if (!user && !PUBLIC.includes(path)) router.replace('/login');
  }, [user, path]);
  return null;
}

export default function RootLayout() {
  const [loaded] = useFonts({
    Inter_400Regular,
    Inter_500Medium,
    Inter_600SemiBold,
    PlusJakartaSans_700Bold,
    PlusJakartaSans_800ExtraBold,
  });
  if (!loaded) return <View style={{ flex: 1, backgroundColor: colors.brand }} />;
  return (
    <SafeAreaProvider>
      <SessionProvider>
        <StatusBar style="dark" />
        <Guard />
        <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg }, animation: 'slide_from_right' }}>
          <Stack.Screen name="index" options={{ animation: 'fade' }} />
          <Stack.Screen name="home" options={{ animation: 'none' }} />
          <Stack.Screen name="feed" options={{ animation: 'none' }} />
          <Stack.Screen name="overview" options={{ animation: 'none' }} />
          <Stack.Screen name="bills" options={{ animation: 'none' }} />
          <Stack.Screen name="profile" options={{ animation: 'none' }} />
        </Stack>
      </SessionProvider>
    </SafeAreaProvider>
  );
}
