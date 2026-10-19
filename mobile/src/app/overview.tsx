import { Text, View } from 'react-native';
import { Caption, Card, CategoryIcon, ErrorBox, H1, H4, Loading, Overline, Progress, Row, Screen } from '../components/ui';
import { Category, Money } from '../lib/api';
import { money, monthName, todayISO, updatedAgo } from '../lib/format';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type Summary = {
  from: string;
  income: Money;
  expense: Money;
  net: Money;
  byCategory: { category: Category; amount: Money; sharePct: number; changeVsPreviousPct: number | null }[];
  dataUpdatedAt: string | null;
};

// 2.3 Overview
export default function Overview() {
  const { data, error, loading, refreshing, refresh, reload } = useApi<Summary>('/summary?period=month');
  const year = todayISO().slice(0, 4);
  return (
    <Screen tabs onRefresh={refresh} refreshing={refreshing}>
      <View>
        <H1 style={{ fontSize: 24 }}>Overview</H1>
        <Caption>{data ? `${monthName(data.from)} ${year}` : ''} · {updatedAgo(data?.dataUpdatedAt)}</Caption>
      </View>
      {loading && !data ? <Loading /> : null}
      {error && !data ? <ErrorBox message={error} onRetry={reload} /> : null}
      {data ? (
        <>
          <View style={{ backgroundColor: colors.brand, borderRadius: 18, padding: 18, gap: 4 }}>
            <Overline style={{ color: colors.onBrandMuted }}>Net this month</Overline>
            <Text style={{ color: '#fff', fontFamily: fonts.headingXB, fontSize: 36, letterSpacing: -1, lineHeight: 44 }}>{money(data.net, { sign: true })}</Text>
            <Row style={{ gap: 26, marginTop: 6 }}>
              <View>
                <Caption style={{ color: colors.onBrandMuted }}>Income</Caption>
                <Text style={{ color: '#fff', fontFamily: fonts.semibold, fontSize: 14 }}>{money(data.income, { sign: true })}</Text>
              </View>
              <View>
                <Caption style={{ color: colors.onBrandMuted }}>Spent</Caption>
                <Text style={{ color: '#fff', fontFamily: fonts.semibold, fontSize: 14 }}>{money(data.expense)}</Text>
              </View>
            </Row>
          </View>
          <H4 style={{ fontSize: 17 }}>Where it went</H4>
          <Card style={{ gap: 14 }}>
            {data.byCategory.length === 0 ? <Caption>No spending this month yet.</Caption> : null}
            {data.byCategory.map((c) => (
              <Row key={c.category.key} style={{ gap: 12, alignItems: 'flex-start' }}>
                <CategoryIcon keyName={c.category.key} size={34} />
                <View style={{ flex: 1, gap: 5 }}>
                  <Row style={{ justifyContent: 'space-between' }}>
                    <Text style={{ fontFamily: fonts.medium, fontSize: 15, color: colors.text }}>{c.category.name}</Text>
                    <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{money({ ...c.amount, amountMinor: Math.abs(c.amount.amountMinor) })}</Text>
                  </Row>
                  <Progress pct={c.sharePct} />
                  <Caption>
                    {c.sharePct}% of spending
                    {c.changeVsPreviousPct !== null ? ` · ${c.changeVsPreviousPct > 0 ? '+' : ''}${c.changeVsPreviousPct}% vs last month` : ''}
                  </Caption>
                </View>
              </Row>
            ))}
          </Card>
        </>
      ) : null}
    </Screen>
  );
}
