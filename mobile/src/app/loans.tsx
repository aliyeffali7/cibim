import { useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import { Back, Body, Button, Caption, Card, CategoryIcon, Empty, ErrorBox, H1, KV, Loading, Progress, Row, Screen } from '../components/ui';
import { api, Account, Money } from '../lib/api';
import { longDate, money } from '../lib/format';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

type Loan = {
  id: string;
  name: string;
  lender: string;
  source: string;
  monthlyPayment: Money;
  nextPaymentDate: string | null;
  remaining: Money | null;
  payoffDate: string | null;
  paymentsSeen: number;
  payingAccountId: string | null;
};

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

// 6.9 Loans
export default function Loans() {
  const { data, error, loading, reload } = useApi<Loan[]>('/loans');
  const accounts = useApi<Account[]>('/accounts');
  return (
    <Screen>
      <Back />
      <View style={{ gap: 6 }}>
        <H1 style={{ fontSize: 26 }}>Loans</H1>
        <Body style={{ color: colors.textSecondary }}>Banks don't share loan details through Open Banking yet. Cibim spots them from your monthly instalments.</Body>
      </View>
      {loading && !data ? <Loading /> : null}
      {error ? <ErrorBox message={error} onRetry={reload} /> : null}
      {data && !data.length ? <Empty text="No loan instalments found." /> : null}
      {data?.map((l) => (
        <LoanCard key={l.id} loan={l} account={accounts.data?.find((a) => a.id === l.payingAccountId)?.name} onSaved={reload} />
      ))}
    </Screen>
  );
}

function LoanCard({ loan: l, account, onSaved }: { loan: Loan; account?: string; onSaved: () => void }) {
  const [left, setLeft] = useState('');
  const [saving, setSaving] = useState(false);
  const paid = l.monthlyPayment.amountMinor * l.paymentsSeen;
  const pct = l.remaining ? (paid / (paid + l.remaining.amountMinor)) * 100 : 0;
  const payoff = l.payoffDate ? `${MONTHS[Number(l.payoffDate.slice(5, 7)) - 1]} ${l.payoffDate.slice(0, 4)}` : null;

  const save = async () => {
    const minor = Math.round(parseFloat(left.replace(',', '.').replace(/\s/g, '')) * 100);
    if (!(minor > 0)) return;
    setSaving(true);
    try {
      await api('PATCH', `/loans/${l.id}`, { remaining: { amountMinor: minor, currency: 'AZN' } });
      onSaved();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card style={{ gap: 10 }}>
      <Row style={{ gap: 12 }}>
        <CategoryIcon keyName="loans" size={38} />
        <View style={{ flex: 1 }}>
          <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{l.name}</Text>
          <Caption>{l.source === 'manual' ? 'Added by you' : `Found from ${l.paymentsSeen} instalments`}</Caption>
        </View>
      </Row>
      {l.remaining ? (
        <>
          <Text style={{ fontFamily: fonts.headingXB, fontSize: 30, color: colors.text }}>{money(l.remaining)}</Text>
          <Caption>left to pay</Caption>
          <Progress pct={pct} />
        </>
      ) : (
        <View style={{ gap: 8 }}>
          <Caption>How much is left to pay? Cibim will count down from it.</Caption>
          <Row style={{ gap: 8 }}>
            <TextInput
              value={left}
              onChangeText={setLeft}
              placeholder="2 610.00"
              keyboardType="decimal-pad"
              placeholderTextColor={colors.textMuted}
              style={{ flex: 1, backgroundColor: colors.bg, borderRadius: 12, paddingHorizontal: 14, height: 44, fontSize: 15, color: colors.text }}
            />
            <Button small label="Save" loading={saving} onPress={save} />
          </Row>
        </View>
      )}
      <View>
        <KV label="Monthly payment" value={money(l.monthlyPayment)} />
        {l.nextPaymentDate ? <KV label="Next payment" value={longDate(l.nextPaymentDate)} /> : null}
        {payoff ? <KV label="Paid off" value={payoff} /> : null}
        {account ? <KV label="Paid from" value={account} /> : null}
      </View>
      {l.remaining ? <Caption>You added the amount left; Cibim counts down from it.</Caption> : null}
    </Card>
  );
}
