import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { Back, Body, Button, Caption, Card, CategoryIcon, Empty, ErrorBox, H1, Loading, Pill, Row, Screen } from '../components/ui';
import { Connection } from '../lib/api';
import { longDate } from '../lib/format';
import { connectBank, useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

// 7.2 Connected banks
export default function Banks() {
  const { data, error, loading, reload } = useApi<Connection[]>('/connections');
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  const renew = async (c: Connection) => {
    setBusy(c.id);
    setMsg(null);
    try {
      const status = await connectBank(c.bank.code, c.id);
      setMsg(status === 'active' ? `${c.bank.name} renewed for 90 days.` : `${c.bank.name} renewal not completed.`);
      await reload();
    } catch (e: any) {
      setMsg(e.message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <Screen footer={<Button label="Connect another bank" variant="secondary" onPress={() => router.push('/connect')} />}>
      <Back />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Connected banks</H1>
        <Body style={{ color: colors.textSecondary }}>Cibim reads these with your permission. Access lasts up to 90 days.</Body>
      </View>
      {msg ? <Caption style={{ color: colors.brand }}>{msg}</Caption> : null}
      {loading && !data ? <Loading /> : null}
      {error ? <ErrorBox message={error} onRetry={reload} /> : null}
      {data && !data.length ? <Empty text="No banks connected yet." /> : null}
      {data?.length ? (
        <Card style={{ paddingVertical: 6 }}>
          {data.map((c) => {
            const bad = c.status !== 'active';
            const sub =
              c.status === 'active'
                ? `${c.accountsCount} account${c.accountsCount === 1 ? '' : 's'} · valid until ${c.validUntil ? longDate(c.validUntil) : '—'}`
                : c.status === 'expiring'
                  ? `${c.accountsCount} account${c.accountsCount === 1 ? '' : 's'} · ends in ${c.daysLeft} days`
                  : c.status === 'pending'
                    ? 'Waiting for approval'
                    : c.status === 'expired'
                      ? 'Access expired'
                      : 'Access stopped. Connect again.';
            return (
              <Row key={c.id} style={{ gap: 12, paddingVertical: 10 }}>
                <CategoryIcon keyName="bank" size={38} tint={bad ? colors.negative : colors.brand} bg={bad ? colors.negativeSoft : colors.accentSoft} />
                <View style={{ flex: 1 }}>
                  <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{c.bank.name}</Text>
                  <Caption>{sub}</Caption>
                </View>
                {c.status === 'active' ? <Pill label="Active" tone="positive" /> : <Button small label="Renew" loading={busy === c.id} onPress={() => renew(c)} />}
              </Row>
            );
          })}
        </Card>
      ) : null}
    </Screen>
  );
}
