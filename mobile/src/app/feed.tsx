import { useMemo, useState } from 'react';
import { ScrollView, View } from 'react-native';
import { TxRow } from '../components/TxRow';
import { Caption, Card, Chip, Empty, ErrorBox, H1, Loading, Row, Screen } from '../components/ui';
import { Account, Transaction } from '../lib/api';
import { dayHeader, money, updatedAgo } from '../lib/format';
import { useApi } from '../lib/session';

type Page = { items: Transaction[]; nextCursor: string | null; dataUpdatedAt: string | null };

// 2.2 Feed (+ 8.4 bank not responding)
export default function Feed() {
  const [filter, setFilter] = useState('all');
  const accounts = useApi<Account[]>('/accounts');
  const banks = useMemo(() => {
    const seen = new Map<string, string>();
    accounts.data?.forEach((a) => seen.set(a.bank.code, a.bank.name));
    return [...seen.entries()];
  }, [accounts.data]);

  const query =
    filter === 'all' ? '' : filter === 'out' ? '&direction=out&includeTransfers=false' : filter === 'in' ? '&direction=in&includeTransfers=false' : `&bankCode=${filter}`;
  const { data, error, loading, refreshing, refresh, reload } = useApi<Page>(`/transactions?limit=200${query}`);

  const groups = useMemo(() => {
    const out: { date: string; items: Transaction[]; total: number }[] = [];
    for (const t of data?.items ?? []) {
      let g = out[out.length - 1];
      if (!g || g.date !== t.date) {
        g = { date: t.date, items: [], total: 0 };
        out.push(g);
      }
      g.items.push(t);
      if (!t.isInternalTransfer) g.total += t.amountAzn.amountMinor;
    }
    return out;
  }, [data]);

  return (
    <Screen tabs onRefresh={refresh} refreshing={refreshing}>
      <View>
        <H1 style={{ fontSize: 24 }}>Transactions</H1>
        <Caption>All banks · {updatedAgo(data?.dataUpdatedAt)}</Caption>
      </View>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={{ gap: 8 }} style={{ flexGrow: 0 }}>
        <Chip label="All" active={filter === 'all'} onPress={() => setFilter('all')} />
        <Chip label="Spending" active={filter === 'out'} onPress={() => setFilter('out')} />
        <Chip label="Income" active={filter === 'in'} onPress={() => setFilter('in')} />
        {banks.map(([code, name]) => (
          <Chip key={code} label={name} active={filter === code} onPress={() => setFilter(code)} />
        ))}
      </ScrollView>
      {error ? <ErrorBox message={`The bank isn't responding right now. Showing what we had. (${error})`} onRetry={reload} /> : null}
      {loading && !data ? <Loading /> : null}
      {data && !data.items.length ? <Empty text="No transactions yet." /> : null}
      {groups.map((g) => (
        <View key={g.date} style={{ gap: 8 }}>
          <Row style={{ justifyContent: 'space-between' }}>
            <Caption>{dayHeader(g.date)}</Caption>
            <Caption>{money({ amountMinor: g.total, currency: 'AZN' }, { sign: true })}</Caption>
          </Row>
          <Card style={{ paddingVertical: 4 }}>
            {g.items.map((t) => (
              <TxRow key={t.id} t={t} showDate={false} />
            ))}
          </Card>
        </View>
      ))}
    </Screen>
  );
}
