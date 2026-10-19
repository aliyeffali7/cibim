import { useLocalSearchParams } from 'expo-router';
import { Text, View } from 'react-native';
import { TxRow } from '../../components/TxRow';
import { Back, Card, Empty, ErrorBox, KV, Loading, Overline, Screen, Section } from '../../components/ui';
import { Account, Connection, Transaction } from '../../lib/api';
import { longDate, money, shortIban, updatedAgo } from '../../lib/format';
import { useApi } from '../../lib/session';
import { colors, fonts } from '../../lib/theme';

// 3.x Account detail
export default function AccountScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const acc = useApi<Account & { dataUpdatedAt: string | null }>(`/accounts/${id}`);
  const conns = useApi<Connection[]>('/connections');
  const txs = useApi<{ items: Transaction[] }>(`/transactions?accountId=${id}&limit=100`);
  const a = acc.data;
  const conn = conns.data?.find((c) => c.id === a?.connectionId);
  const expiring = conn && conn.daysLeft !== null && conn.daysLeft <= 7;
  const label = a ? `${a.bank.name} · ${a.type === 'CARD' ? 'Card' : a.type === 'CURRENT' ? `${a.balance.currency} current account` : 'Savings'}${a.maskedPan ? ` •• ${a.maskedPan.slice(-4)}` : ''}` : '';

  return (
    <Screen onRefresh={txs.refresh} refreshing={txs.refreshing}>
      <Back label="Home" />
      {acc.loading && !a ? <Loading /> : null}
      {acc.error && !a ? <ErrorBox message={acc.error} onRetry={acc.reload} /> : null}
      {a ? (
        <>
          <View style={{ backgroundColor: colors.brand, borderRadius: 18, padding: 18, gap: 4 }}>
            <Overline style={{ color: colors.onBrandMuted }}>{label}</Overline>
            <Text style={{ color: '#fff', fontFamily: fonts.headingXB, fontSize: 36, letterSpacing: -1, lineHeight: 44 }}>{money(a.balance)}</Text>
            <Text style={{ color: colors.onBrandMuted, fontFamily: fonts.body, fontSize: 13 }}>
              {a.balance.currency !== 'AZN' ? `≈ ${money(a.balanceAzn)} at today's CBAR rate · ` : 'Available now · '}
              {updatedAgo(a.dataUpdatedAt)}
            </Text>
          </View>
          <Card style={{ paddingVertical: 8 }}>
            {a.maskedPan ? <KV label="Card" value={`${a.maskedPan.slice(0, 4)} •••• •••• ${a.maskedPan.slice(-4)}`} /> : <KV label="Type" value="Current account" />}
            <KV label="IBAN" value={shortIban(a.iban ?? '')} />
            {conn?.validUntil ? (
              <KV
                label="Access valid until"
                value={`${longDate(conn.validUntil)}${expiring ? ` · ${conn.daysLeft} days left` : ''}`}
                valueColor={expiring ? colors.negative : undefined}
              />
            ) : null}
          </Card>
          <Section title="Transactions">
            {txs.data && !txs.data.items.length ? <Empty text="No transactions in the last 90 days." /> : null}
            {txs.data?.items.length ? (
              <Card style={{ paddingVertical: 4 }}>
                {txs.data.items.map((t) => (
                  <TxRow key={t.id} t={t} />
                ))}
              </Card>
            ) : null}
          </Section>
        </>
      ) : null}
    </Screen>
  );
}
