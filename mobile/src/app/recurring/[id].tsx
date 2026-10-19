import { useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { Back, Caption, Card, CategoryIcon, ErrorBox, KV, Loading, Pill, Row, Screen, Section, Toggle } from '../../components/ui';
import { api, Account, Recurring, Transaction } from '../../lib/api';
import { longDate, money, shortDate } from '../../lib/format';
import { useApi } from '../../lib/session';
import { colors, fonts } from '../../lib/theme';

// 6.x Recurring bill or income
export default function RecurringScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const list = useApi<Recurring[]>('/recurring');
  const txs = useApi<{ items: Transaction[] }>(`/transactions?recurringId=${id}&limit=12`);
  const accounts = useApi<Account[]>('/accounts');
  const r = list.data?.find((x) => x.id === id);
  const [regular, setRegular] = useState(true);
  useEffect(() => {
    if (r) setRegular(r.confirmed || r.source === 'detected');
  }, [r?.id]);

  const items = txs.data?.items ?? [];
  const last = items[0];
  const paidFrom = accounts.data?.find((a) => a.id === last?.accountId);
  const income = r?.type === 'income';
  const lastAmount = last ? Math.abs(last.amountAzn.amountMinor) : null;
  const typical = r ? Math.abs(r.typicalAmount.amountMinor) : 0;
  const changePct = r?.status === 'changed' && lastAmount && typical ? Math.round(((lastAmount - typical) / typical) * 100) : null;

  const toggle = async (v: boolean) => {
    setRegular(v);
    try {
      await api('PATCH', `/recurring/${id}`, { confirmed: v });
    } catch {
      setRegular(!v);
    }
  };

  return (
    <Screen>
      <Back />
      {list.loading && !list.data ? <Loading /> : null}
      {list.error ? <ErrorBox message={list.error} onRetry={list.reload} /> : null}
      {list.data && !r ? <ErrorBox message="This item is no longer detected." /> : null}
      {r ? (
        <>
          <View style={{ alignItems: 'center', gap: 4, marginTop: 4 }}>
            <CategoryIcon keyName={r.category.key} size={56} />
            <Text style={{ fontFamily: fonts.heading, fontSize: 18, color: colors.text, marginTop: 6 }}>{r.name}</Text>
            <Caption>
              {r.cadence === 'weekly' ? 'Weekly' : 'Monthly'} {income ? 'income' : 'bill'} · {income ? 'expected' : 'due'} {shortDate(r.nextDate)}
            </Caption>
            <Text style={{ fontFamily: fonts.headingXB, fontSize: 32, color: income ? colors.positive : colors.text }}>
              {money(last ? last.amountAzn : r.typicalAmount, { sign: true })}
            </Text>
            {changePct !== null ? <Pill label={`${changePct > 0 ? 'Up' : 'Down'} ${Math.abs(changePct)}% on usual`} tone="warning" /> : null}
            {r.status === 'late' ? <Pill label="Late" tone="negative" /> : null}
          </View>
          <Card style={{ paddingVertical: 8 }}>
            <KV label={income ? 'Next expected' : 'Next payment'} value={longDate(r.nextDate)} />
            <KV label="Usual amount" value={money({ amountMinor: typical, currency: 'AZN' })} />
            {paidFrom ? <KV label={income ? 'Paid into' : 'Paid from'} value={paidFrom.name} /> : null}
            <KV label="Found from" value={r.source === 'manual' ? 'Added by you' : `${items.length} payments`} />
          </Card>
          {items.length ? (
            <Section title={income ? 'Last payments in' : 'Last payments'}>
              <Card style={{ paddingVertical: 6 }}>
                {items.map((t) => (
                  <Row key={t.id} style={{ justifyContent: 'space-between', paddingVertical: 7 }}>
                    <Caption style={{ fontSize: 14 }}>{shortDate(t.date)}</Caption>
                    <Text style={{ fontFamily: fonts.semibold, color: colors.text }}>{money(t.amountAzn, { sign: true })}</Text>
                  </Row>
                ))}
              </Card>
            </Section>
          ) : null}
          <Card>
            <Row style={{ gap: 12 }}>
              <CategoryIcon keyName="internal_transfer" size={36} />
              <View style={{ flex: 1 }}>
                <Text style={{ fontFamily: fonts.medium, fontSize: 15, color: colors.text }}>This is a regular {income ? 'income' : 'bill'}</Text>
                <Caption>Keeps it in your calendar and safe-to-spend.</Caption>
              </View>
              <Toggle value={regular} onChange={toggle} />
            </Row>
          </Card>
        </>
      ) : null}
    </Screen>
  );
}
