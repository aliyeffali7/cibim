import { router } from 'expo-router';
import { Text, View } from 'react-native';
import { Caption, Card, CategoryIcon, Empty, ErrorBox, H1, ListRow, Loading, Overline, Row, Screen } from '../components/ui';
import { Money, Recurring } from '../lib/api';
import { daysFromToday, money, monthName, shortDate } from '../lib/format';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type Calendar = {
  days: { date: string; items: { recurringId: string; name: string; type: string; amount: Money }[]; net: Money }[];
  totals: { income: Money; expense: Money; net: Money };
};

// 2.4 Bills & income
export default function Bills() {
  const cal = useApi<Calendar>('/recurring/calendar?days=30');
  const rec = useApi<Recurring[]>('/recurring');
  const catOf = (id: string) => rec.data?.find((r) => r.id === id)?.category.key ?? 'utilities';

  const buckets: { title: string; rows: { date: string; item: Calendar['days'][0]['items'][0] }[] }[] = [];
  const add = (title: string, date: string, item: Calendar['days'][0]['items'][0]) => {
    let b = buckets.find((x) => x.title === title);
    if (!b) buckets.push((b = { title, rows: [] }));
    b.rows.push({ date, item });
  };
  for (const d of cal.data?.days ?? []) {
    const n = daysFromToday(d.date);
    const title = n < 7 ? 'This week' : n < 14 ? 'Next week' : `Later in ${monthName(d.date)}`;
    d.items.forEach((it) => add(title, d.date, it));
  }

  return (
    <Screen tabs onRefresh={cal.refresh} refreshing={cal.refreshing}>
      <View>
        <H1 style={{ fontSize: 24 }}>Bills & income</H1>
        <Caption>Next 30 days · detected from your transactions</Caption>
      </View>
      {cal.loading && !cal.data ? <Loading /> : null}
      {cal.error && !cal.data ? <ErrorBox message={cal.error} onRetry={cal.reload} /> : null}
      {cal.data ? (
        <>
          <Row style={{ gap: 10 }}>
            <Card style={{ flex: 1, backgroundColor: colors.positiveSoft }}>
              <Caption>Coming in</Caption>
              <Text style={{ fontFamily: fonts.heading, fontSize: 20, color: colors.positive }}>{money(cal.data.totals.income, { sign: true })}</Text>
            </Card>
            <Card style={{ flex: 1, backgroundColor: colors.negativeSoft }}>
              <Caption>Going out</Caption>
              <Text style={{ fontFamily: fonts.heading, fontSize: 20, color: colors.negative }}>{money(cal.data.totals.expense)}</Text>
            </Card>
          </Row>
          {buckets.length === 0 ? <Empty text="Nothing recurring found yet." /> : null}
          {buckets.map((b) => (
            <View key={b.title} style={{ gap: 8 }}>
              <Overline>{b.title}</Overline>
              <Card style={{ paddingVertical: 6 }}>
                {b.rows.map(({ date, item }) => (
                  <ListRow
                    key={item.recurringId + date}
                    onPress={() => router.push(`/recurring/${item.recurringId}`)}
                    icon={<CategoryIcon keyName={catOf(item.recurringId)} size={36} />}
                    title={item.name}
                    subtitle={`${item.type === 'income' ? 'Expected' : 'Due'} ${shortDate(date)}`}
                    value={money(item.amount, { sign: true })}
                    valueColor={item.amount.amountMinor > 0 ? colors.positive : undefined}
                  />
                ))}
              </Card>
            </View>
          ))}
          <Card onPress={() => router.push('/loans')}>
            <ListRow icon={<CategoryIcon keyName="loans" size={36} />} title="Loans" subtitle="Instalments Cibim found in your payments" chevron />
          </Card>
        </>
      ) : null}
    </Screen>
  );
}
