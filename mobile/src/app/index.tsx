import { router } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { Text, View } from 'react-native';
import { Logo } from '../components/Logo';
import { colors, fonts } from '../lib/theme';

// 1.1 Splash
export default function Splash() {
  useEffect(() => {
    const t = setTimeout(() => router.replace('/login'), 1300);
    return () => clearTimeout(t);
  }, []);
  return (
    <View style={{ flex: 1, backgroundColor: colors.brand, alignItems: 'center', justifyContent: 'center' }}>
      <StatusBar style="light" />
      <Logo color={colors.loginBg} size={64} />
      <Text style={{ position: 'absolute', bottom: 56, color: colors.onBrandMuted, fontFamily: fonts.medium, fontSize: 15 }}>
        Every bank. One view.
      </Text>
    </View>
  );
}
