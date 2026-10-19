import { Ionicons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, ScrollView, Text, View } from 'react-native';
import {
  Body,
  Button,
  Caption,
  Card,
  CategoryIcon,
  ErrorBox,
  H1,
  H4,
  ListRow,
  Loading,
  Overline,
  Row,
  Screen,
  Section,
} from '../components/ui';
import { Home as HomeData, Recurring } from '../lib/api';
import { last4, money, shortDate, updatedAgo } from '../lib/format';
import { alertIcon, openAlertTarget } from '../lib/nav';
import { useApi, useSession } from '../lib/session';
import { colors, fonts } from '../lib/theme';

// 2.1 Home (+ 8.1 no banks, 8.2 loading, 8.3 bank access expired)
export default function Home() {
  const { user } = useSession();
  const { data, error, loading, refreshing, refresh, reload } = useApi<HomeData>('/home');
  const recurring = useApi<Recurring[]>('/recurring');
  const unread = data?.alerts.length ?? 0;
  const catOf = (id: string, type: string) => recurring.data?.find((r) => r.id === id)?.category.key ?? (type === 'income' ? 'salary' : 'utilities');

  return (
    <Screen tabs onRefresh={refresh} refreshing={refreshing}>
      <Row style={{ justifyContent: 'space-between' }}>
        <View>
          <H1 style={{ fontSize: 24 }}>Salam, {user?.name}</H1>
          <Caption>{updatedAgo(data?.dataUpdatedAt)}</Caption>
        </View>
        <Pressable onPress={() => router.push('/alerts')} style={{ width: 40, height: 40, borderRadius: 20, backgroundColor: '#fff', alignItems: 'center', justifyContent: 'center' }}>
          <Ionicons name="notifications-outline" size={20} color={colors.text} />
          {unread > 0 ? <View style={{ position: 'absolute', top: 9, right: 10, width: 8, height: 8, borderRadius: 4, backgroundColor: colors.negative }} /> : null}
        </Pressable>
      </Row>

      {loading && !data ? <Loading /> : null}
      {error && !data ? <ErrorBox message={error} onRetry={reload} /> : null}

      {data?.onboarding ? (
        <Card style={{ alignItems: 'center', gap: 10, paddingVertical: 28 }}>
          <CategoryIcon keyName="bank" size={56} />
          <H4 style={{ fontSize: 18 }}>No banks yet</H4>
          <Body style={{ color: colors.textSecondary, textAlign: 'center' }}>Connect a bank to see every card, balance and expense in one place.</Body>
          <View style={{ alignSelf: 'stretch', marginTop: 6 }}>
            <Button label="Connect a bank" onPress={() => router.push('/connect')} />
          </View>
        </Card>
      ) : null}

      {data && !data.onboarding ? (
        <>
          {/* Total balance */}
          <View style={{ backgroundColor: colors.brand, borderRadius: 18, padding: 18, gap: 4 }}>
            <Overline style={{ color: colors.onBrandMuted }}>Total balance · {data.accounts.length} accounts</Overline>
            <Text style={{ color: '#fff', fontFamily: fonts.headingXB, fontSize: 38, letterSpacing: -1, lineHeight: 46 }}>{money(data.totalBalance)}</Text>
            <Row style={{ gap: 22, marginTop: 8 }}>
              {(
                [
                  ['Today', data.spending?.today],
                  ['This week', data.spending?.thisWeek],
                  ['This month', data.spending?.thisMonth],
                ] as const
              ).map(([label, m]) => (
                <View key={label}>
                  <Caption style={{ color: colors.onBrandMuted }}>{label}</Caption>
                  <Text style={{ color: '#fff', fontFamily: fonts.semibold, fontSize: 14 }}>{money(m)}</Text>
                </View>
              ))}
            </Row>
          </View>

          {/* Safe to spend + streak */}
          <Row style={{ gap: 10, alignItems: 'stretch' }}>
            <Card style={{ flex: 1, gap: 2 }}>
              <Row style={{ gap: 4 }}>
                <Ionicons name="shield-checkmark-outline" size={14} color={colors.textSecondary} />
                <Caption>Safe to spend today</Caption>
              </Row>
              <Text style={{ fontFamily: fonts.heading, fontSize: 22, color: colors.positive }}>{money(data.safeToSpend?.today)}</Text>
              <Caption>
                {data.safeToSpend?.nextIncome ? `Until ${data.safeToSpend.nextIncome.name.toLowerCase()} · ${shortDate(data.safeToSpend.nextIncome.date)}` : 'Until month end'}
              </Caption>
            </Card>
            <Pressable onPress={() => router.push('/alerts')} style={{ flex: 1, backgroundColor: colors.highlight, borderRadius: 16, padding: 16, gap: 2 }}>
              <Row style={{ gap: 4 }}>
                <Ionicons name="flame-outline" size={14} color={colors.brand} />
                <Caption style={{ color: colors.brand }}>Streak</Caption>
              </Row>
              <Text style={{ fontFamily: fonts.heading, fontSize: 22, color: colors.brand }}>{data.streak?.current ?? 0} days</Text>
              <Caption style={{ color: colors.brand }}>Best: {data.streak?.best ?? 0} days</Caption>
            </Pressable>
          </Row>

          {/* Plan a purchase */}
          <Card onPress={() => router.push('/plan')} style={{ backgroundColor: colors.highlightSoft }}>
            <Row style={{ gap: 12 }}>
              <View style={{ width: 36, height: 36, borderRadius: 10, backgroundColor: colors.highlight, alignItems: 'center', justifyContent: 'center' }}>
                <Ionicons name="scale-outline" size={18} color={colors.brand} />
              </View>
              <View style={{ flex: 1 }}>
                <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>Plan a purchase</Text>
                <Caption>Check it against your limit before you buy.</Caption>
              </View>
              <Ionicons name="chevron-forward" size={18} color={colors.textSecondary} />
            </Row>
          </Card>

          {/* Top alert */}
          {data.alerts[0] ? (
            <Card
              onPress={() => openAlertTarget(data.alerts[0].action.screen, data.alerts[0].action.targetId)}
              style={{ backgroundColor: alertIcon(data.alerts[0].type)[2] }}
            >
              <Row style={{ gap: 10, alignItems: 'flex-start' }}>
                <Ionicons name={alertIcon(data.alerts[0].type)[0] as any} size={18} color={alertIcon(data.alerts[0].type)[1]} />
                <View style={{ flex: 1 }}>
                  <Text style={{ fontFamily: fonts.semibold, fontSize: 14.5, color: colors.text }}>{data.alerts[0].fallbackText.title}</Text>
                  <Caption>{data.alerts[0].fallbackText.body}</Caption>
                </View>
              </Row>
            </Card>
          ) : null}

          {/* Accounts */}
          <Section title="Accounts">
            <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={{ gap: 10, paddingRight: 8 }} style={{ marginHorizontal: -18, paddingLeft: 18 }}>
              {data.accounts.map((a) => (
                <Card key={a.id} onPress={() => router.push(`/account/${a.id}`)} style={{ width: 150, gap: 2 }}>
                  <Caption style={{ color: colors.text }}>{a.bank.name}</Caption>
                  <Caption numberOfLines={1}>{a.type === 'CARD' ? `${a.name.split(' ')[1] ?? 'Card'} ${last4(a.maskedPan)}` : `${a.balance.currency} account`}</Caption>
                  <Text style={{ fontFamily: fonts.heading, fontSize: 17, color: colors.text, marginTop: 14 }}>{money(a.balance)}</Text>
                </Card>
              ))}
            </ScrollView>
          </Section>

          {/* Upcoming */}
          {data.upcoming.length ? (
            <Section title="Upcoming">
              <Card style={{ paddingVertical: 6 }}>
                {data.upcoming.map((u) => (
                  <ListRow
                    key={u.recurringId + u.date}
                    onPress={() => router.push(`/recurring/${u.recurringId}`)}
                    icon={<CategoryIcon keyName={catOf(u.recurringId, u.type)} size={36} />}
                    title={u.name}
                    subtitle={`${u.type === 'income' ? 'Expected' : 'Due'} ${shortDate(u.date)}`}
                    value={money(u.amount, { sign: true })}
                    valueColor={u.amount.amountMinor > 0 ? colors.positive : undefined}
                  />
                ))}
              </Card>
            </Section>
          ) : null}
        </>
      ) : null}
    </Screen>
  );
}
