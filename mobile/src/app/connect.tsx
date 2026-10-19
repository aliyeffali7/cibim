import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { Back, Body, Button, Card, Caption, CategoryIcon, ErrorBox, H1, Loading, Pill, Row, Screen } from '../components/ui';
import { api } from '../lib/api';
import { connectBank, useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type BankItem = { code: string; name: string; connected: boolean };

// 1.3 Connect banks (+ 8.5 Approval cancelled)
export default function Connect() {
  const banks = useApi<BankItem[]>('/banks');
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<{ tone: 'ok' | 'bad'; text: string } | null>(null);

  const connect = async (b: BankItem) => {
    setBusy(b.code);
    setMessage(null);
    try {
      const status = await connectBank(b.code);
      if (status === 'active' || status === 'expiring') {
        setMessage({ tone: 'ok', text: `${b.name} connected. Your data is ready.` });
      } else {
        setMessage({ tone: 'bad', text: `${b.name} approval was cancelled. Nothing was shared. You can try again.` });
      }
      await banks.reload();
    } catch (e: any) {
      setMessage({ tone: 'bad', text: e.message });
    } finally {
      setBusy(null);
    }
  };

  const connectAll = async () => {
    setBusy('ALL');
    try {
      await api('POST', '/demo/connect-all');
      router.replace('/home');
    } catch (e: any) {
      setMessage({ tone: 'bad', text: e.message });
    } finally {
      setBusy(null);
    }
  };

  const anyConnected = banks.data?.some((b) => b.connected);

  return (
    <Screen
      footer={
        anyConnected ? <Button label="Continue" onPress={() => router.replace('/home')} /> : undefined
      }
    >
      <Back />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Connect your banks</H1>
        <Body style={{ color: colors.textSecondary }}>Pick a bank, then approve in its app. Cibim can only read your data. It can never move money.</Body>
      </View>
      {message ? (
        message.tone === 'ok' ? (
          <Card style={{ backgroundColor: colors.positiveSoft }}>
            <Row style={{ gap: 8 }}>
              <Ionicons name="checkmark-circle" size={20} color={colors.positive} />
              <Body style={{ flex: 1, color: colors.positive }}>{message.text}</Body>
            </Row>
          </Card>
        ) : (
          <ErrorBox message={message.text} />
        )
      ) : null}
      {banks.loading ? <Loading /> : null}
      {banks.error ? <ErrorBox message={banks.error} onRetry={banks.reload} /> : null}
      {banks.data ? (
        <Card style={{ paddingVertical: 6 }}>
          {banks.data.map((b) => (
            <Row key={b.code} style={{ gap: 12, paddingVertical: 10 }}>
              <CategoryIcon keyName="bank" size={36} />
              <Text style={{ flex: 1, fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{b.name}</Text>
              {b.connected ? (
                <Pill label="Connected" tone="positive" />
              ) : (
                <Button small label="Connect" loading={busy === b.code} disabled={!!busy} onPress={() => connect(b)} />
              )}
            </Row>
          ))}
        </Card>
      ) : null}
      <Card style={{ backgroundColor: colors.accentSoft }}>
        <Row style={{ gap: 10, alignItems: 'flex-start' }}>
          <Ionicons name="shield-checkmark-outline" size={18} color={colors.brand} />
          <Caption style={{ flex: 1, color: colors.text }}>Access lasts up to 90 days. Cibim reminds you before it runs out.</Caption>
        </Row>
      </Card>
      <Text onPress={busy ? undefined : connectAll} style={{ textAlign: 'center', color: colors.textSecondary, fontFamily: fonts.medium, fontSize: 13, paddingVertical: 6 }}>
        {busy === 'ALL' ? 'Connecting…' : 'Demo shortcut: connect all my banks'}
      </Text>
    </Screen>
  );
}
