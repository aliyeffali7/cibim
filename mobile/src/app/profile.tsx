import { router } from 'expo-router';
import { Text, View } from 'react-native';
import { Caption, Card, CategoryIcon, H2, ListRow, Overline, Row, Screen, Toggle } from '../components/ui';
import { api, Budget, Connection, User } from '../lib/api';
import { shortDate } from '../lib/format';
import { useApi, useSession } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type Loan = { id: string; nextPaymentDate: string | null };

// 2.5 Profile
export default function Profile() {
  const { user, setUser, logout } = useSession();
  const conns = useApi<Connection[]>('/connections');
  const budgets = useApi<Budget[]>('/budgets');
  const loans = useApi<Loan[]>('/loans');

  const update = async (patch: Partial<User> & { profile?: User['profile'] }) => {
    const u = await api<User>('PATCH', '/me', patch);
    setUser(u);
  };

  const expiring = conns.data?.find((c) => c.status === 'expiring' || c.status === 'expired');
  const banksLine = conns.data?.length
    ? conns.data.map((c) => c.bank.name).join(', ') + (expiring ? ` · ${expiring.bank.name} ${expiring.status === 'expired' ? 'expired' : `expires in ${expiring.daysLeft} days`}` : '')
    : 'No banks yet';
  const over = budgets.data?.filter((b) => b.status === 'over').length ?? 0;
  const nextLoan = loans.data?.find((l) => l.nextPaymentDate);

  if (!user) return null;
  return (
    <Screen tabs>
      <Row style={{ gap: 12, marginTop: 4 }}>
        <View style={{ width: 52, height: 52, borderRadius: 26, backgroundColor: colors.brand, alignItems: 'center', justifyContent: 'center' }}>
          <Text style={{ color: '#fff', fontFamily: fonts.heading, fontSize: 20 }}>{user.name[0]}</Text>
        </View>
        <View>
          <H2>{user.name}</H2>
          <Caption>Demo user · {user.id}</Caption>
        </View>
      </Row>

      <Overline>Your setup</Overline>
      <Card style={{ paddingVertical: 6 }}>
        <ListRow icon={<CategoryIcon keyName="bank" size={36} />} title="Connected banks" subtitle={banksLine} chevron onPress={() => router.push('/banks')} />
        <ListRow
          icon={<CategoryIcon keyName="assets" size={36} />}
          title="Budgets and limits"
          subtitle={`${budgets.data?.length ?? 0} limits${over ? ` · ${over} over` : ''}`}
          chevron
          onPress={() => router.push('/budgets')}
        />
        <ListRow
          icon={<CategoryIcon keyName="loans" size={36} />}
          title="Loans"
          subtitle={`${loans.data?.length ?? 0} loan${loans.data?.length === 1 ? '' : 's'}${nextLoan?.nextPaymentDate ? ` · next payment ${shortDate(nextLoan.nextPaymentDate)}` : ''}`}
          chevron
          onPress={() => router.push('/loans')}
        />
      </Card>

      <Overline>About you</Overline>
      <Card style={{ gap: 12 }}>
        <Row style={{ gap: 12 }}>
          <CategoryIcon keyName="home" size={36} />
          <View style={{ flex: 1 }}>
            <Text style={{ fontFamily: fonts.medium, fontSize: 15, color: colors.text }}>I run a household</Text>
            <Caption>Turns on seasonal reminders, like Novruz and New Year.</Caption>
          </View>
          <Toggle value={user.profile.householder} onChange={(v) => update({ profile: { ...user.profile, householder: v } })} />
        </Row>
        <Row style={{ gap: 12 }}>
          <CategoryIcon keyName="school" size={36} />
          <View style={{ flex: 1 }}>
            <Text style={{ fontFamily: fonts.medium, fontSize: 15, color: colors.text }}>School-age children</Text>
            <Caption>Reminds you to plan for school supplies in September.</Caption>
          </View>
          <Toggle value={user.profile.schoolAgeChildren} onChange={(v) => update({ profile: { ...user.profile, schoolAgeChildren: v } })} />
        </Row>
      </Card>

      <Card onPress={() => update({ language: user.language === 'en' ? 'az' : 'en' })}>
        <ListRow icon={<CategoryIcon keyName="other" size={36} />} title="Language" value={user.language === 'en' ? 'English' : 'Azərbaycanca'} />
      </Card>

      <Card
        onPress={() => {
          logout();
          router.replace('/login');
        }}
      >
        <Text style={{ color: colors.negative, fontFamily: fonts.semibold, fontSize: 15, paddingVertical: 4 }}>Log out</Text>
      </Card>
    </Screen>
  );
}
