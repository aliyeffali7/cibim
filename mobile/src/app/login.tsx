import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';
import { Logo } from '../components/Logo';
import { Body, Caption, ErrorBox, H1, Loading, Screen } from '../components/ui';
import { api, Home } from '../lib/api';
import { useSession } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type DemoUser = { psuId: string; name: string; persona: string };

const ROLE: Record<string, string> = { 'u-aysel': 'Salaried', 'u-rashad': 'Householder', 'u-nigar': 'Student' };

// 1.2 Login: pick a demo user
export default function Login() {
  const { login } = useSession();
  const [users, setUsers] = useState<DemoUser[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const load = () => {
    setError(null);
    api<DemoUser[]>('GET', '/auth/demo-users').then(setUsers).catch((e) => setError(e.message));
  };
  useEffect(load, []);

  const pick = async (u: DemoUser) => {
    setBusy(u.psuId);
    try {
      await login(u.psuId);
      const home = await api<Home>('GET', '/home');
      router.replace(home.onboarding ? '/connect' : '/home');
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <Screen bg={colors.loginBg} footer={<Caption>Data comes from the mock CBAR Open Banking server.</Caption>}>
      <View style={{ marginTop: 28, marginBottom: 12 }}>
        <Logo size={40} />
      </View>
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Pick a demo user</H1>
        <Body style={{ color: colors.textSecondary }}>Hackathon demo. Real sign-up comes after.</Body>
      </View>
      {error ? <ErrorBox message={`Can't reach Cibim: ${error}`} onRetry={load} /> : null}
      {!users && !error ? <Loading /> : null}
      <View style={{ gap: 10 }}>
        {users?.map((u) => (
          <Pressable
            key={u.psuId}
            onPress={() => pick(u)}
            style={({ pressed }) => ({ backgroundColor: '#fff', borderRadius: 16, padding: 14, flexDirection: 'row', alignItems: 'center', gap: 12, opacity: pressed ? 0.8 : 1 })}
          >
            <View style={{ width: 40, height: 40, borderRadius: 20, backgroundColor: colors.brand, alignItems: 'center', justifyContent: 'center' }}>
              <Text style={{ color: '#fff', fontFamily: fonts.semibold, fontSize: 16 }}>{u.name[0]}</Text>
            </View>
            <View style={{ flex: 1 }}>
              <Text style={{ fontFamily: fonts.semibold, fontSize: 16, color: colors.text }}>{u.name}</Text>
              <Caption numberOfLines={1}>{ROLE[u.psuId] ?? u.persona}</Caption>
            </View>
            {busy === u.psuId ? <ActivityIndicator color={colors.brand} /> : <Ionicons name="chevron-forward" size={18} color={colors.textMuted} />}
          </Pressable>
        ))}
      </View>
    </Screen>
  );
}
