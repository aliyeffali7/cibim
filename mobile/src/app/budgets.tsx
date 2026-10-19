import { router } from 'expo-router';
import { Text, View } from 'react-native';
import { Back, Body, Button, Caption, Card, CategoryIcon, ErrorBox, H1, Loading, Pill, Progress, Row, Screen } from '../components/ui';
import { Budget } from '../lib/api';
import { amount, periodLabel, shortDate } from '../lib/format';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

// 5.2 Budgets and limits
export default function Budgets() {
  const { data, error, loading, reload } = useApi<Budget[]>('/budgets');
  return (
    <Screen footer={<Button label="Add a limit" variant="secondary" onPress={() => router.push('/budget-edit')} />}>
      <Back />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Budgets and limits</H1>
        <Body style={{ color: colors.textSecondary }}>Cibim warns you at 80% and tells you when you pass 100%.</Body>
      </View>
      {loading && !data ? <Loading /> : null}
      {error ? <ErrorBox message={error} onRetry={reload} /> : null}
      {data?.map((b) => {
        const tone = b.status === 'over' ? 'negative' : b.status === 'warning' ? 'warning' : 'positive';
        const color = { negative: colors.negative, warning: colors.warning, positive: colors.positive }[tone];
        const bar = b.status === 'over' ? colors.negative : b.status === 'warning' ? colors.warning : colors.brand;
        const key = b.category?.key ?? 'total';
        return (
          <Card key={b.id} style={{ gap: 10 }}>
            <Row style={{ gap: 12 }}>
              <CategoryIcon keyName={key} size={38} />
              <View style={{ flex: 1 }}>
                <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{b.category?.name ?? 'All spending'}</Text>
                <Caption>
                  {periodLabel(b.period)} limit{b.basis === 'baseline' ? ' · suggested from your average' : ''}
                </Caption>
              </View>
              <Pill tone={tone} label={b.status === 'over' ? 'Over' : b.status === 'warning' ? 'Close' : 'On track'} />
            </Row>
            <Progress pct={b.pct} color={bar} />
            <Row style={{ justifyContent: 'space-between' }}>
              <Caption style={{ fontSize: 14 }}>
                {amount(b.spent.amountMinor)} ₼ of {amount(b.limit.amountMinor)} ₼
              </Caption>
              <Text style={{ fontFamily: fonts.semibold, color }}>{Math.round(b.pct)}%</Text>
            </Row>
            {b.alreadySpent.amountMinor > 0 && b.dataSince ? (
              <Caption>
                Based on data since {shortDate(b.dataSince)}, plus {amount(b.alreadySpent.amountMinor)} ₼ you added.
              </Caption>
            ) : null}
            <Text
              onPress={() => router.push({ pathname: '/budget-edit', params: { id: b.id, category: b.category?.key ?? '' } })}
              style={{ color: colors.brand, fontFamily: fonts.semibold, fontSize: 14 }}
            >
              {b.basis === 'baseline' ? 'Set my own limit' : 'Edit limit'}
            </Text>
          </Card>
        );
      })}
    </Screen>
  );
}
