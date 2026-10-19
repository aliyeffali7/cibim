import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { Back, Button, Caption, Card, categoryIconSpec, CategoryIcon, Chip, ErrorBox, KV, Loading, Pill, Row, Screen, Section, Toggle } from '../../components/ui';
import { api, Account, Category, Transaction } from '../../lib/api';
import { longDate, money } from '../../lib/format';
import { useApi } from '../../lib/session';
import { colors, fonts } from '../../lib/theme';

const CHOICES = ['dining', 'groceries', 'transport', 'shopping', 'clothes', 'subscriptions', 'utilities', 'health', 'tech', 'home', 'other'];

// 4.x Transaction detail: change its category
export default function TransactionScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const tx = useApi<Transaction>(`/transactions/${id}`);
  const accounts = useApi<Account[]>('/accounts');
  const cats = useApi<Category[]>('/categories');
  const [cat, setCat] = useState<string | null>(null);
  const [all, setAll] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const t = tx.data;
  useEffect(() => {
    if (t && cat === null) setCat(t.category.key);
  }, [t]);

  const account = accounts.data?.find((a) => a.id === t?.accountId);
  const choices = t && !CHOICES.includes(t.category.key) ? [t.category.key, ...CHOICES] : CHOICES;
  const nameOf = (k: string) => cats.data?.find((c) => c.key === k)?.name ?? k;
  const changed = t && cat && cat !== t.category.key;

  const save = async () => {
    if (!t || !cat) return;
    setSaving(true);
    try {
      await api('PATCH', `/transactions/${t.id}`, { categoryId: `cat-${cat}`, applyToMerchant: all });
      router.back();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Screen footer={t ? <Button label="Save" onPress={save} loading={saving} disabled={!changed && !all} /> : undefined}>
      <Back />
      {tx.loading && !t ? <Loading /> : null}
      {tx.error && !t ? <ErrorBox message={tx.error} onRetry={tx.reload} /> : null}
      {t ? (
        <>
          <View style={{ alignItems: 'center', gap: 6, marginTop: 4 }}>
            <CategoryIcon keyName={t.category.key} size={56} />
            <Text style={{ fontFamily: fonts.heading, fontSize: 18, color: colors.text, marginTop: 4 }}>{t.merchant.name}</Text>
            <Text style={{ fontFamily: fonts.headingXB, fontSize: 32, color: t.amount.amountMinor > 0 ? colors.positive : colors.text }}>{money(t.amount, { sign: true })}</Text>
            {t.status === 'pending' ? <Pill label="Pending" tone="warning" /> : null}
            {t.isInternalTransfer ? <Pill label="Between your accounts · not counted" /> : null}
          </View>
          <Card style={{ paddingVertical: 8 }}>
            <KV label="Date" value={longDate(t.date)} />
            <KV label="Account" value={account ? account.name : t.bank.name} />
            <KV label="Bank description" value={t.rawDescription} />
          </Card>
          {error ? <ErrorBox message={error} /> : null}
          <Section title="Category">
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
              {choices.map((k) => (
                <Chip key={k} label={nameOf(k)} icon={categoryIconSpec(k)} active={cat === k} onPress={() => setCat(k)} />
              ))}
            </View>
          </Section>
          <Card>
            <Row style={{ gap: 12 }}>
              <CategoryIcon keyName="internal_transfer" size={36} />
              <View style={{ flex: 1 }}>
                <Text style={{ fontFamily: fonts.medium, fontSize: 15, color: colors.text }}>Apply to all {t.merchant.name} payments</Text>
                <Caption>Now and in future</Caption>
              </View>
              <Toggle value={all} onChange={setAll} />
            </Row>
          </Card>
        </>
      ) : null}
    </Screen>
  );
}
